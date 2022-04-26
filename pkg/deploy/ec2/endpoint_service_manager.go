package ec2

import (
	"context"
	"time"

	awssdk "github.com/aws/aws-sdk-go/aws"
	ec2sdk "github.com/aws/aws-sdk-go/service/ec2"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/aws-load-balancer-controller/pkg/aws/services"
	"sigs.k8s.io/aws-load-balancer-controller/pkg/deploy/tracking"
	ec2model "sigs.k8s.io/aws-load-balancer-controller/pkg/model/ec2"
	"sigs.k8s.io/aws-load-balancer-controller/pkg/networking"
)

// abstraction around endpoint service operations for EC2.
type EndpointServiceManager interface {
	// ReconcileTags will reconcile tags on resources.
	ReconcileTags(ctx context.Context, resID string, desiredTags map[string]string, opts ...ReconcileTagsOption) error

	// ListEndpointServices returns VPC Endpoint Services that matches any of the tagging requirements.
	ListEndpointServices(ctx context.Context, tagFilters ...tracking.TagFilter) ([]ec2model.VPCEndpointService, error)

	Create(ctx context.Context, resES *ec2model.VPCEndpointService) (ec2model.VPCEndpointServiceStatus, error)

	Update(ctx context.Context, resES *ec2model.VPCEndpointService, sdkES networking.VPCEndpointServiceInfo) (ec2model.VPCEndpointServiceStatus, error)

	Delete(ctx context.Context, sdkES networking.VPCEndpointServiceInfo) error

	ReconcilePermissions(ctx context.Context, permissions *ec2model.VPCEndpointServicePermissions) error
}

// NewdefaultEndpointServiceManager constructs new defaultEndpointServiceManager.
func NewDefaultEndpointServiceManager(ec2Client services.EC2, vpcID string, logger logr.Logger, trackingProvider tracking.Provider) *defaultEndpointServiceManager {
	return &defaultEndpointServiceManager{
		ec2Client:        ec2Client,
		vpcID:            vpcID,
		logger:           logger,
		trackingProvider: trackingProvider,
	}
}

var _ EndpointServiceManager = &defaultEndpointServiceManager{}

// default implementation for EndpointServiceManager.
type defaultEndpointServiceManager struct {
	ec2Client        services.EC2
	vpcID            string
	logger           logr.Logger
	trackingProvider tracking.Provider

	waitESDeletionPollInterval time.Duration
	waitESDeletionTimeout      time.Duration
}

func (m *defaultEndpointServiceManager) ReconcileTags(ctx context.Context, resID string, desiredTags map[string]string, opts ...ReconcileTagsOption) error {
	return nil
}

func (m *defaultEndpointServiceManager) ListEndpointServices(ctx context.Context, tagFilters ...tracking.TagFilter) ([]ec2model.VPCEndpointService, error) {
	return nil, nil
}

func (m *defaultEndpointServiceManager) Create(ctx context.Context, resSG *ec2model.VPCEndpointService) (ec2model.VPCEndpointServiceStatus, error) {
	sgTags := m.trackingProvider.ResourceTags(resSG.Stack(), resSG, resSG.Spec.Tags)
	sdkTags := convertTagsToSDKTags(sgTags)

	var resolvedLoadBalancerArns []string
	for _, unresolved := range resSG.Spec.NetworkLoadBalancerArns {
		arn, err := unresolved.Resolve(ctx)
		if err != nil {
			return ec2model.VPCEndpointServiceStatus{}, err
		}
		resolvedLoadBalancerArns = append(resolvedLoadBalancerArns, arn)
	}

	var privateDnsName *string
	if resSG.Spec.PrivateDNSName != nil {
		privateDnsName = awssdk.String(*resSG.Spec.PrivateDNSName)
	}

	req := ec2sdk.CreateVpcEndpointServiceConfigurationInput{
		AcceptanceRequired:      awssdk.Bool(*resSG.Spec.AcceptanceRequired),
		PrivateDnsName:          privateDnsName,
		NetworkLoadBalancerArns: awssdk.StringSlice(resolvedLoadBalancerArns),
		TagSpecifications: []*ec2sdk.TagSpecification{
			{
				ResourceType: awssdk.String("vpc-endpoint-service"),
				Tags:         sdkTags,
			},
		},
	}
	m.logger.Info("creating VpcEndpointService",
		"resourceID", resSG.ID())
	resp, err := m.ec2Client.CreateVpcEndpointServiceConfigurationWithContext(ctx, &req)
	if err != nil {
		return ec2model.VPCEndpointServiceStatus{}, err
	}
	serviceID := awssdk.StringValue(resp.ServiceConfiguration.ServiceId)
	m.logger.Info("created VpcEndpointService",
		"resourceID", resSG.ID(),
		"serviceID", serviceID)

	return ec2model.VPCEndpointServiceStatus{
		ServiceID: serviceID,
	}, nil
}

func (m *defaultEndpointServiceManager) Update(ctx context.Context, resES *ec2model.VPCEndpointService, sdkES networking.VPCEndpointServiceInfo) (ec2model.VPCEndpointServiceStatus, error) {

	m.logger.Info("Updating", "resES", resES, "sdkES", sdkES)

	var resLBArnsRaw []string
	for _, lb := range resES.Spec.NetworkLoadBalancerArns {
		arn, err := lb.Resolve(ctx)
		if err != nil {
			return ec2model.VPCEndpointServiceStatus{}, err
		}
		resLBArnsRaw = append(resLBArnsRaw, arn)
	}

	sdkLBArns := sets.NewString(sdkES.NetworkLoadBalancerArns...)
	resLBArns := sets.NewString(resLBArnsRaw...)

	// TODO move this to algorithm
	var addLBArns, removeLBArns []*string
	for _, arn := range resLBArns.Difference(sdkLBArns).List() {
		addLBArns = append(addLBArns, &arn)
	}
	for _, arn := range sdkLBArns.Difference(resLBArns).List() {
		removeLBArns = append(removeLBArns, &arn)
	}

	var acceptanceRequired *bool
	if *resES.Spec.AcceptanceRequired != sdkES.AcceptanceRequired {
		acceptanceRequired = resES.Spec.AcceptanceRequired
	}

	var privateDNSName *string
	var removePrivateDNSName *bool
	if resES.Spec.PrivateDNSName == nil && sdkES.PrivateDNSName != nil {
		removePrivateDNSName = newBoolPointer(true)
	} else if resES.Spec.PrivateDNSName != sdkES.PrivateDNSName {
		privateDNSName = resES.Spec.PrivateDNSName
	}

	if len(addLBArns) > 0 || len(removeLBArns) > 0 || acceptanceRequired != nil || privateDNSName != nil || removePrivateDNSName != nil {

		serviceId := &sdkES.ServiceID

		m.logger.Info(
			"Updating VPCEndpointService",
			"addLBArns", addLBArns,
			"removeLBArns", removeLBArns,
			"acceptanceRequired", acceptanceRequired,
			"privateDNSName", privateDNSName,
			"removePrivateDNSName", removePrivateDNSName,
			"serviceId", serviceId,
		)

		req := ec2sdk.ModifyVpcEndpointServiceConfigurationInput{
			AcceptanceRequired:            acceptanceRequired,
			AddNetworkLoadBalancerArns:    addLBArns,
			RemoveNetworkLoadBalancerArns: removeLBArns,
			PrivateDnsName:                privateDNSName,
			RemovePrivateDnsName:          removePrivateDNSName,
			ServiceId:                     serviceId,
		}

		_, err := m.ec2Client.ModifyVpcEndpointServiceConfigurationWithContext(ctx, &req)
		if err != nil {
			return ec2model.VPCEndpointServiceStatus{}, err
		}
	} else {
		m.logger.Info(
			"Not updating VPCEndpointService",
		)
	}

	return ec2model.VPCEndpointServiceStatus{
		ServiceID: sdkES.ServiceID,
	}, nil
}

func (m *defaultEndpointServiceManager) Delete(ctx context.Context, sdkES networking.VPCEndpointServiceInfo) error {
	req := &ec2sdk.DeleteVpcEndpointServiceConfigurationsInput{
		ServiceIds: awssdk.StringSlice(
			[]string{sdkES.ServiceID},
		),
	}

	m.logger.Info("deleting VPCEndpointService",
		"serviceId", sdkES.ServiceID)
	if _, err := m.ec2Client.DeleteVpcEndpointServiceConfigurationsWithContext(ctx, req); err != nil {
		return err
	}
	m.logger.Info("deleted VPCEndpointService",
		"serviceId", sdkES.ServiceID)

	return nil
}

func (m *defaultEndpointServiceManager) ReconcilePermissions(ctx context.Context, permissions *ec2model.VPCEndpointServicePermissions) error {
	m.logger.Info("Reconciling Permissions")

	serviceId, err := permissions.Spec.ServiceId.Resolve(ctx)
	if err != nil {
		m.logger.Info("Failed to resolve serviceId", "err", err)
		return err
	}
	req := &ec2sdk.DescribeVpcEndpointServicePermissionsInput{
		ServiceId: &serviceId,
	}

	m.logger.Info("Reconciling Permissions for service", "serviceId", serviceId)

	permissionsInfo, err := m.fetchESPermissionInfosFromAWS(ctx, req)
	if err != nil {
		m.logger.Info("Error while fetching existing VPC endpoint service permissions")
		return err
	}
	sdkPrinciples := sets.NewString(permissionsInfo.AllowedPrincipals...)
	resPrinciples := sets.NewString(permissions.Spec.AllowedPrinciples...)

	// TODO move this to algorithm
	var addPrinciples, removePrinciples []*string
	for _, principle := range resPrinciples.Difference(sdkPrinciples).List() {
		addPrinciples = append(addPrinciples, &principle)
	}
	for _, principle := range sdkPrinciples.Difference(resPrinciples).List() {
		removePrinciples = append(removePrinciples, &principle)
	}

	modReq := &ec2sdk.ModifyVpcEndpointServicePermissionsInput{
		AddAllowedPrincipals:    addPrinciples,
		RemoveAllowedPrincipals: removePrinciples,
		ServiceId:               &serviceId,
	}

	m.logger.Info("Build priciples",
		"AddPrinciples", addPrinciples,
		"RemovePrinciples", removePrinciples,
	)

	if len(addPrinciples) > 0 || len(removePrinciples) > 0 {

		m.logger.Info("modifying VpcEndpointService permissions",
			"serviceID", serviceId,
			"addPrinciples", addPrinciples,
			"removePrinciples", removePrinciples,
		)

		_, err := m.ec2Client.ModifyVpcEndpointServicePermissionsWithContext(ctx, modReq)
		if err != nil {
			return err
		}

		m.logger.Info("modified VpcEndpointService permissions",
			"serviceID", serviceId)
	}

	return nil
}

func (m *defaultEndpointServiceManager) fetchESPermissionInfosFromAWS(ctx context.Context, req *ec2sdk.DescribeVpcEndpointServicePermissionsInput) (networking.VPCEndpointServicePermissionsInfo, error) {
	endpointServicePermissions, err := m.ec2Client.DescribeVpcEndpointServicePermissionsWithContext(ctx, req)
	if err != nil {
		return networking.VPCEndpointServicePermissionsInfo{}, errors.Wrap(err, "Failed to fetch VPCEndpointPermissions from AWS")
	}
	return networking.NewRawVPCEndpointServicePermissionsInfo(endpointServicePermissions), nil
}

func newBoolPointer(value bool) *bool {
	b := value
	return &b
}

package ec2

import (
	"context"

	awssdk "github.com/aws/aws-sdk-go/aws"
	ec2sdk "github.com/aws/aws-sdk-go/service/ec2"
	"github.com/go-logr/logr"
	"sigs.k8s.io/aws-load-balancer-controller/pkg/aws/services"
	"sigs.k8s.io/aws-load-balancer-controller/pkg/deploy/tracking"
	ec2model "sigs.k8s.io/aws-load-balancer-controller/pkg/model/ec2"
)

// abstraction around endpoint service operations for EC2.
type EndpointServiceManager interface {
	// ReconcileTags will reconcile tags on resources.
	ReconcileTags(ctx context.Context, resID string, desiredTags map[string]string, opts ...ReconcileTagsOption) error

	// ListEndpointServices returns VPC Endpoint Services that matches any of the tagging requirements.
	ListEndpointServices(ctx context.Context, tagFilters ...tracking.TagFilter) ([]ec2model.VPCEndpointService, error)

	Create(ctx context.Context, resSG *ec2model.VPCEndpointService) (ec2model.VPCEndpointServiceStatus, error)
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
	// TODO
	// permissionInfos, err := buildIPPermissionInfos(resSG.Spec.Ingress)
	// if err != nil {
	// 	return ec2model.VPCEndpointServiceStatus{}, err
	// }

	var resolvedLoadBalancerArns []string
	for _, unresolved := range resSG.Spec.NetworkLoadBalancerArns {
		arn, err := unresolved.Resolve(ctx)
		if err != nil {
			return ec2model.VPCEndpointServiceStatus{}, err
		}
		resolvedLoadBalancerArns = append(resolvedLoadBalancerArns, arn)
	}

	req := ec2sdk.CreateVpcEndpointServiceConfigurationInput{
		AcceptanceRequired:      awssdk.Bool(*resSG.Spec.AcceptanceRequired),
		PrivateDnsName:          awssdk.String(*resSG.Spec.PrivateDNSName),
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
	sgID := awssdk.StringValue(resp.ServiceConfiguration.ServiceId)
	m.logger.Info("created VpcEndpointService",
		"resourceID", resSG.ID(),
		"serviceID", sgID)

	// TODO
	// Do we need to reconcile here?

	return ec2model.VPCEndpointServiceStatus{
		ServiceID: sgID,
	}, nil
}

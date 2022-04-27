package ec2

import (
	"context"
	"errors"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go/aws"
	ec2sdk "github.com/aws/aws-sdk-go/service/ec2"
	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/aws-load-balancer-controller/pkg/aws/services"
	"sigs.k8s.io/aws-load-balancer-controller/pkg/deploy/tracking"
	"sigs.k8s.io/aws-load-balancer-controller/pkg/model/core"
	"sigs.k8s.io/aws-load-balancer-controller/pkg/model/ec2"
	ec2model "sigs.k8s.io/aws-load-balancer-controller/pkg/model/ec2"
	"sigs.k8s.io/aws-load-balancer-controller/pkg/networking"
)

type testStringToken struct {
	core.Token
	value string
	err   error
}

func (t testStringToken) Resolve(ctx context.Context) (string, error) {
	return t.value, t.err
}

type DescribeVpcEndpointServicePermissionsWithContextResponse struct {
	response *ec2sdk.DescribeVpcEndpointServicePermissionsOutput
	err      error
}

func Test_Update_responses(t *testing.T) {
	lbArn := "lbArn"
	privateDNSName := "http://example.com"
	serviceID := "serviceID"
	ctx := context.TODO()

	tests := []struct {
		name               string
		nlbResolveError    error
		modifyAPICallError error
		shouldError        bool
	}{
		{
			name:               "returns an error when the service id can't be resolved",
			nlbResolveError:    errors.New("test_error"),
			modifyAPICallError: nil,
			shouldError:        true,
		},
		{
			name:               "returns an error when the API call returns an error",
			nlbResolveError:    nil,
			modifyAPICallError: errors.New("test_error"),
			shouldError:        true,
		},
		{
			name:               "returns correctly with no errors",
			nlbResolveError:    nil,
			modifyAPICallError: nil,
			shouldError:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := &ec2model.VPCEndpointService{
				Spec: ec2model.VPCEndpointServiceSpec{
					AcceptanceRequired: newBoolPointer(false),
					NetworkLoadBalancerArns: []core.StringToken{
						testStringToken{
							value: lbArn,
							err:   tt.nlbResolveError,
						},
					},
					PrivateDNSName: &privateDNSName,
				},
			}
			sdk := networking.VPCEndpointServiceInfo{
				ServiceID: serviceID,
			}

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			mockEC2 := services.NewMockEC2(mockCtrl)
			if tt.nlbResolveError == nil {
				mockEC2.EXPECT().ModifyVpcEndpointServiceConfigurationWithContext(ctx, gomock.Any()).Return(
					// We don't use this value
					nil,
					tt.modifyAPICallError,
				).Times(1)
			}

			manager := NewDefaultEndpointServiceManager(
				mockEC2,
				"vpcID",
				logr.DiscardLogger{},
				tracking.NewDefaultProvider("", ""),
			)

			resp, err := manager.Update(ctx, res, sdk)

			if tt.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, resp.ServiceID, serviceID)
			}
		})
	}
}

func Test_Update_modifyVPCEndpointServiceConfigurationInput(t *testing.T) {
	lbArn := "lbArn"
	privateDNSName := "http://example.com"
	serviceID := "serviceID"
	ctx := context.TODO()

	tests := []struct {
		name string
		res  *ec2model.VPCEndpointService
		sdk  networking.VPCEndpointServiceInfo
		req  *ec2sdk.ModifyVpcEndpointServiceConfigurationInput
	}{
		{
			name: "AcceptanceRequired gets set in input",
			res: &ec2model.VPCEndpointService{
				Spec: ec2model.VPCEndpointServiceSpec{
					AcceptanceRequired: newBoolPointer(true),
				},
			},
			sdk: networking.VPCEndpointServiceInfo{
				AcceptanceRequired: false,
				ServiceID:          serviceID,
			},
			req: &ec2sdk.ModifyVpcEndpointServiceConfigurationInput{
				AcceptanceRequired:            newBoolPointer(true),
				AddNetworkLoadBalancerArns:    nil,
				RemoveNetworkLoadBalancerArns: nil,
				PrivateDnsName:                nil,
				RemovePrivateDnsName:          nil,
				ServiceId:                     &serviceID,
			},
		},
		{
			name: "AddNetworkLoadBalancerArns gets set in input",
			res: &ec2model.VPCEndpointService{
				Spec: ec2model.VPCEndpointServiceSpec{
					NetworkLoadBalancerArns: []core.StringToken{
						testStringToken{
							value: lbArn,
							err:   nil,
						},
					},
				},
			},
			sdk: networking.VPCEndpointServiceInfo{
				ServiceID: serviceID,
			},
			req: &ec2sdk.ModifyVpcEndpointServiceConfigurationInput{
				AcceptanceRequired:            nil,
				AddNetworkLoadBalancerArns:    []*string{&lbArn},
				RemoveNetworkLoadBalancerArns: nil,
				PrivateDnsName:                nil,
				RemovePrivateDnsName:          nil,
				ServiceId:                     &serviceID,
			},
		},
		{
			name: "RemoveNetworkLoadBalancerArns gets set in input",
			res: &ec2model.VPCEndpointService{
				Spec: ec2model.VPCEndpointServiceSpec{},
			},
			sdk: networking.VPCEndpointServiceInfo{
				NetworkLoadBalancerArns: []string{lbArn},
				ServiceID:               serviceID,
			},
			req: &ec2sdk.ModifyVpcEndpointServiceConfigurationInput{
				AcceptanceRequired:            nil,
				AddNetworkLoadBalancerArns:    nil,
				RemoveNetworkLoadBalancerArns: []*string{&lbArn},
				PrivateDnsName:                nil,
				RemovePrivateDnsName:          nil,
				ServiceId:                     &serviceID,
			},
		},
		{
			name: "PrivateDnsName gets set in input",
			res: &ec2model.VPCEndpointService{
				Spec: ec2model.VPCEndpointServiceSpec{
					PrivateDNSName: &privateDNSName,
				},
			},
			sdk: networking.VPCEndpointServiceInfo{
				ServiceID: serviceID,
			},
			req: &ec2sdk.ModifyVpcEndpointServiceConfigurationInput{
				AcceptanceRequired:            nil,
				AddNetworkLoadBalancerArns:    nil,
				RemoveNetworkLoadBalancerArns: nil,
				PrivateDnsName:                &privateDNSName,
				RemovePrivateDnsName:          nil,
				ServiceId:                     &serviceID,
			},
		},
		{
			name: "RemovePrivateDnsName gets set in input",
			res: &ec2model.VPCEndpointService{
				Spec: ec2model.VPCEndpointServiceSpec{},
			},
			sdk: networking.VPCEndpointServiceInfo{
				PrivateDNSName: &privateDNSName,
				ServiceID:      serviceID,
			},
			req: &ec2sdk.ModifyVpcEndpointServiceConfigurationInput{
				AcceptanceRequired:            nil,
				AddNetworkLoadBalancerArns:    nil,
				RemoveNetworkLoadBalancerArns: nil,
				PrivateDnsName:                nil,
				RemovePrivateDnsName:          newBoolPointer(true),
				ServiceId:                     &serviceID,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			mockEC2 := services.NewMockEC2(mockCtrl)
			mockEC2.EXPECT().ModifyVpcEndpointServiceConfigurationWithContext(ctx, gomock.Eq(tt.req)).Return(
				// We don't use this value
				nil,
				nil,
			).Times(1)

			manager := NewDefaultEndpointServiceManager(
				mockEC2,
				"vpcID",
				logr.DiscardLogger{},
				tracking.NewDefaultProvider("", ""),
			)

			resp, err := manager.Update(ctx, tt.res, tt.sdk)

			assert.NoError(t, err)
			assert.Equal(t, resp.ServiceID, serviceID)
		})
	}
}

func Test_Delete(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	serviceID := "serviceID"
	sdkES := networking.VPCEndpointServiceInfo{
		ServiceID: serviceID,
	}

	ctx := context.TODO()

	tests := []struct {
		name                       string
		deleteResponseError        error
		waitESDeletionPollInterval time.Duration
		waitESDeletionTimeout      time.Duration
	}{
		{
			name:                "calls delete with expected arguments",
			deleteResponseError: nil,
		},
		{
			name:                "returns an error if the delete call returns an error",
			deleteResponseError: errors.New("test_error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockEC2 := services.NewMockEC2(mockCtrl)
			manager := NewDefaultEndpointServiceManager(
				mockEC2,
				"vpcID",
				logr.DiscardLogger{},
				tracking.NewDefaultProvider("", ""),
			)
			req := &ec2sdk.DeleteVpcEndpointServiceConfigurationsInput{
				ServiceIds: awssdk.StringSlice(
					[]string{serviceID},
				),
			}

			mockEC2.EXPECT().DeleteVpcEndpointServiceConfigurationsWithContext(ctx, gomock.Eq(req)).Return(
				// We never use this return value
				nil,
				tt.deleteResponseError,
			).Times(1)

			err := manager.Delete(ctx, sdkES)

			if tt.deleteResponseError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_ReconcilePermissions(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	principleName := "principle"
	serviceID := "serviceID"

	describeVpcEndpointServicePermissionsWithContextReq := &ec2sdk.DescribeVpcEndpointServicePermissionsInput{
		ServiceId: &serviceID,
	}

	ctx := context.TODO()
	tests := []struct {
		name                                                  string
		desiredAllowedPrinciples                              []string
		describePermissionsResponse                           DescribeVpcEndpointServicePermissionsWithContextResponse
		ModifyVpcEndpointServicePermissionsWithContextRequest *ec2sdk.ModifyVpcEndpointServicePermissionsInput
		ModifyVpcEndpointServicePermissionsWithContextError   error
		expectError                                           bool
	}{
		{
			name:                     "returns an error when describe permissions AWS call returns an error",
			desiredAllowedPrinciples: []string{},
			describePermissionsResponse: DescribeVpcEndpointServicePermissionsWithContextResponse{
				response: &ec2sdk.DescribeVpcEndpointServicePermissionsOutput{},
				err:      errors.New("test_error"),
			},
			ModifyVpcEndpointServicePermissionsWithContextRequest: nil,
			ModifyVpcEndpointServicePermissionsWithContextError:   nil,
			expectError: true,
		},
		{
			name:                     "does not call update when there are no principles to be changed",
			desiredAllowedPrinciples: []string{principleName},
			describePermissionsResponse: DescribeVpcEndpointServicePermissionsWithContextResponse{
				response: &ec2sdk.DescribeVpcEndpointServicePermissionsOutput{
					AllowedPrincipals: []*ec2sdk.AllowedPrincipal{
						{
							Principal: &principleName,
						},
					},
				},
				err: nil,
			},
			ModifyVpcEndpointServicePermissionsWithContextRequest: nil,
			ModifyVpcEndpointServicePermissionsWithContextError:   nil,
			expectError: false,
		},
		{
			name:                     "returns and error when update call returns an error",
			desiredAllowedPrinciples: []string{principleName},
			describePermissionsResponse: DescribeVpcEndpointServicePermissionsWithContextResponse{
				response: &ec2sdk.DescribeVpcEndpointServicePermissionsOutput{
					AllowedPrincipals: []*ec2sdk.AllowedPrincipal{},
				},
				err: nil,
			},
			ModifyVpcEndpointServicePermissionsWithContextRequest: &ec2sdk.ModifyVpcEndpointServicePermissionsInput{
				AddAllowedPrincipals: []*string{&principleName},
				ServiceId:            &serviceID,
			},
			ModifyVpcEndpointServicePermissionsWithContextError: errors.New("test_error"),
			expectError: true,
		},
		{
			name:                     "calls update when a principle need to be added",
			desiredAllowedPrinciples: []string{principleName},
			describePermissionsResponse: DescribeVpcEndpointServicePermissionsWithContextResponse{
				response: &ec2sdk.DescribeVpcEndpointServicePermissionsOutput{
					AllowedPrincipals: []*ec2sdk.AllowedPrincipal{},
				},
				err: nil,
			},
			ModifyVpcEndpointServicePermissionsWithContextRequest: &ec2sdk.ModifyVpcEndpointServicePermissionsInput{
				AddAllowedPrincipals: []*string{&principleName},
				ServiceId:            &serviceID,
			},
			ModifyVpcEndpointServicePermissionsWithContextError: nil,
			expectError: false,
		},
		{
			name:                     "calls update when a principle need to be removed",
			desiredAllowedPrinciples: []string{},
			describePermissionsResponse: DescribeVpcEndpointServicePermissionsWithContextResponse{
				response: &ec2sdk.DescribeVpcEndpointServicePermissionsOutput{
					AllowedPrincipals: []*ec2sdk.AllowedPrincipal{
						{
							Principal: &principleName,
						},
					},
				},
				err: nil,
			},
			ModifyVpcEndpointServicePermissionsWithContextRequest: &ec2sdk.ModifyVpcEndpointServicePermissionsInput{
				RemoveAllowedPrincipals: []*string{&principleName},
				ServiceId:               &serviceID,
			},
			ModifyVpcEndpointServicePermissionsWithContextError: nil,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockEC2 := services.NewMockEC2(mockCtrl)
			manager := NewDefaultEndpointServiceManager(
				mockEC2,
				"vpcID",
				logr.DiscardLogger{},
				tracking.NewDefaultProvider("", ""),
			)

			permissions := &ec2.VPCEndpointServicePermissions{
				Spec: ec2.VPCEndpointServicePermissionsSpec{
					AllowedPrinciples: tt.desiredAllowedPrinciples,
					ServiceId: testStringToken{
						value: serviceID,
					},
				},
			}

			// Set up mocks
			mockEC2.EXPECT().DescribeVpcEndpointServicePermissionsWithContext(ctx, gomock.Eq(describeVpcEndpointServicePermissionsWithContextReq)).Return(
				tt.describePermissionsResponse.response,
				tt.describePermissionsResponse.err,
			).Times(1)
			if tt.ModifyVpcEndpointServicePermissionsWithContextRequest != nil {
				mockEC2.EXPECT().ModifyVpcEndpointServicePermissionsWithContext(
					ctx,
					gomock.Eq(tt.ModifyVpcEndpointServicePermissionsWithContextRequest),
				).Return(
					// We never use this response value
					nil,
					tt.ModifyVpcEndpointServicePermissionsWithContextError,
				).Times(1)
			} else {
				mockEC2.EXPECT().ModifyVpcEndpointServicePermissionsWithContext(gomock.Any(), gomock.Any()).Times(0)
			}

			err := manager.ReconcilePermissions(ctx, permissions)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_fetchESPermissionInfosFromAWS(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	ctx := context.TODO()
	pricipalNames := []string{"principle1", "principle2"}
	req := &ec2sdk.DescribeVpcEndpointServicePermissionsInput{}

	tests := []struct {
		name         string
		mockResponse DescribeVpcEndpointServicePermissionsWithContextResponse
		expected     networking.VPCEndpointServicePermissionsInfo
		err          bool
	}{
		{
			name: "returns valid output on valid request",
			mockResponse: DescribeVpcEndpointServicePermissionsWithContextResponse{
				response: &ec2sdk.DescribeVpcEndpointServicePermissionsOutput{
					AllowedPrincipals: []*ec2sdk.AllowedPrincipal{
						{Principal: &pricipalNames[0]},
						{Principal: &pricipalNames[1]},
					},
				},
				err: nil,
			},
			expected: networking.VPCEndpointServicePermissionsInfo{
				AllowedPrincipals: pricipalNames,
				ServiceId:         "",
			},
			err: false,
		},
		{
			name: "returns an error on an SDK error",
			mockResponse: DescribeVpcEndpointServicePermissionsWithContextResponse{
				response: &ec2sdk.DescribeVpcEndpointServicePermissionsOutput{},
				err:      errors.New("test_error"),
			},
			expected: networking.VPCEndpointServicePermissionsInfo{},
			err:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockEC2 := services.NewMockEC2(mockCtrl)
			manager := NewDefaultEndpointServiceManager(
				mockEC2,
				"vpcID",
				logr.DiscardLogger{},
				tracking.NewDefaultProvider("", ""),
			)
			mockEC2.EXPECT().DescribeVpcEndpointServicePermissionsWithContext(ctx, req).Return(
				tt.mockResponse.response,
				tt.mockResponse.err,
			).Times(1)
			actual, err := manager.fetchESPermissionInfosFromAWS(ctx, req)
			assert.Equal(t, tt.expected, actual)
			if tt.err {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

package ec2

import (
	"context"
	"errors"
	"testing"

	ec2sdk "github.com/aws/aws-sdk-go/service/ec2"
	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/aws-load-balancer-controller/pkg/aws/services"
	"sigs.k8s.io/aws-load-balancer-controller/pkg/deploy/tracking"
	"sigs.k8s.io/aws-load-balancer-controller/pkg/model/core"
	"sigs.k8s.io/aws-load-balancer-controller/pkg/model/ec2"
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

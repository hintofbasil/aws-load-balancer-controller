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
	"sigs.k8s.io/aws-load-balancer-controller/pkg/networking"
)

func Test_fetchESPermissionInfosFromAWS(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	ctx := context.TODO()
	pricipalNames := []string{"principle1", "principle2"}
	req := &ec2sdk.DescribeVpcEndpointServicePermissionsInput{}

	type ec2Response struct {
		response *ec2sdk.DescribeVpcEndpointServicePermissionsOutput
		err      error
	}

	tests := []struct {
		name         string
		mockResponse ec2Response
		expected     networking.VPCEndpointServicePermissionsInfo
		err          error
	}{
		{
			name: "returns valid output on valid request",
			mockResponse: ec2Response{
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
			err: nil,
		},
		{
			name: "returns an error on an SDK error",
			mockResponse: ec2Response{
				response: &ec2sdk.DescribeVpcEndpointServicePermissionsOutput{},
				err:      errors.New("test_error"),
			},
			expected: networking.VPCEndpointServicePermissionsInfo{},
			err:      errors.New("test_error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockEC2 := services.NewMockEC2(mockCtrl)
			manager := NewDefaultEndpointServiceManager(
				mockEC2,
				"test",
				logr.DiscardLogger{},
				tracking.NewDefaultProvider("", ""),
			)
			mockEC2.EXPECT().DescribeVpcEndpointServicePermissionsWithContext(ctx, req).Return(
				tt.mockResponse.response,
				tt.mockResponse.err,
			)
			actual, err := manager.fetchESPermissionInfosFromAWS(ctx, req)
			assert.Equal(t, tt.expected, actual)
			assert.Equal(t, tt.err, err)
		})
	}
}

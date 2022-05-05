// Code generated by MockGen. DO NOT EDIT.
// Source: sigs.k8s.io/aws-load-balancer-controller/pkg/networking (interfaces: VPCEndpointServiceManager)

// Package networking is a generated GoMock package.
package networking

import (
	context "context"
	reflect "reflect"

	ec2 "github.com/aws/aws-sdk-go/service/ec2"
	gomock "github.com/golang/mock/gomock"
)

// MockVPCEndpointServiceManager is a mock of VPCEndpointServiceManager interface.
type MockVPCEndpointServiceManager struct {
	ctrl     *gomock.Controller
	recorder *MockVPCEndpointServiceManagerMockRecorder
}

// MockVPCEndpointServiceManagerMockRecorder is the mock recorder for MockVPCEndpointServiceManager.
type MockVPCEndpointServiceManagerMockRecorder struct {
	mock *MockVPCEndpointServiceManager
}

// NewMockVPCEndpointServiceManager creates a new mock instance.
func NewMockVPCEndpointServiceManager(ctrl *gomock.Controller) *MockVPCEndpointServiceManager {
	mock := &MockVPCEndpointServiceManager{ctrl: ctrl}
	mock.recorder = &MockVPCEndpointServiceManagerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockVPCEndpointServiceManager) EXPECT() *MockVPCEndpointServiceManagerMockRecorder {
	return m.recorder
}

// FetchVPCESInfosByID mocks base method.
func (m *MockVPCEndpointServiceManager) FetchVPCESInfosByID(arg0 context.Context, arg1 []string, arg2 ...FetchVPCESInfoOption) (map[string]VPCEndpointServiceInfo, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "FetchVPCESInfosByID", varargs...)
	ret0, _ := ret[0].(map[string]VPCEndpointServiceInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// FetchVPCESInfosByID indicates an expected call of FetchVPCESInfosByID.
func (mr *MockVPCEndpointServiceManagerMockRecorder) FetchVPCESInfosByID(arg0, arg1 interface{}, arg2 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FetchVPCESInfosByID", reflect.TypeOf((*MockVPCEndpointServiceManager)(nil).FetchVPCESInfosByID), varargs...)
}

// FetchVPCESInfosByRequest mocks base method.
func (m *MockVPCEndpointServiceManager) FetchVPCESInfosByRequest(arg0 context.Context, arg1 *ec2.DescribeVpcEndpointServiceConfigurationsInput) (map[string]VPCEndpointServiceInfo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "FetchVPCESInfosByRequest", arg0, arg1)
	ret0, _ := ret[0].(map[string]VPCEndpointServiceInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// FetchVPCESInfosByRequest indicates an expected call of FetchVPCESInfosByRequest.
func (mr *MockVPCEndpointServiceManagerMockRecorder) FetchVPCESInfosByRequest(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FetchVPCESInfosByRequest", reflect.TypeOf((*MockVPCEndpointServiceManager)(nil).FetchVPCESInfosByRequest), arg0, arg1)
}
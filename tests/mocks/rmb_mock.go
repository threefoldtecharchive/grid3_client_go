// Code generated by MockGen. DO NOT EDIT.
// Source: /home/alaa/zos/pkg/rmb/interface.go

// Package mocks is a generated GoMock package.
package mocks

import (
	context "context"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
)


// RMBMockClient is a mock of Client interface.
type RMBMockClient struct {
	ctrl     *gomock.Controller
	recorder *RMBMockClientMockRecorder
}

// RMBMockClientMockRecorder is the mock recorder for RMBMockClient.
type RMBMockClientMockRecorder struct {
	mock *RMBMockClient
}

// NewRMBMockClient creates a new mock instance.
func NewRMBMockClient(ctrl *gomock.Controller) *RMBMockClient {
	mock := &RMBMockClient{ctrl: ctrl}
	mock.recorder = &RMBMockClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *RMBMockClient) EXPECT() *RMBMockClientMockRecorder {
	return m.recorder
}

// Call mocks base method.
func (m *RMBMockClient) Call(ctx context.Context, twin uint32, fn string, data, result interface{}) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Call", ctx, twin, fn, data, result)
	ret0, _ := ret[0].(error)
	return ret0
}

// Call indicates an expected call of Call.
func (mr *RMBMockClientMockRecorder) Call(ctx, twin, fn, data, result interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Call", reflect.TypeOf((*RMBMockClient)(nil).Call), ctx, twin, fn, data, result)
}


// import (
// 	context "context"
// 	reflect "reflect"

// 	gomock "github.com/golang/mock/gomock"
// 	rmb "github.com/threefoldtech/zos/pkg/rmb"
// )

// // MockRouter is a mock of Router interface.
// type MockRouter struct {
// 	ctrl     *gomock.Controller
// 	recorder *MockRouterMockRecorder
// }

// // MockRouterMockRecorder is the mock recorder for MockRouter.
// type MockRouterMockRecorder struct {
// 	mock *MockRouter
// }

// // NewMockRouter creates a new mock instance.
// func NewMockRouter(ctrl *gomock.Controller) *MockRouter {
// 	mock := &MockRouter{ctrl: ctrl}
// 	mock.recorder = &MockRouterMockRecorder{mock}
// 	return mock
// }

// // EXPECT returns an object that allows the caller to indicate expected use.
// func (m *MockRouter) EXPECT() *MockRouterMockRecorder {
// 	return m.recorder
// }

// // Subroute mocks base method.
// func (m *MockRouter) Subroute(route string) rmb.Router {
// 	m.ctrl.T.Helper()
// 	ret := m.ctrl.Call(m, "Subroute", route)
// 	ret0, _ := ret[0].(rmb.Router)
// 	return ret0
// }

// // Subroute indicates an expected call of Subroute.
// func (mr *MockRouterMockRecorder) Subroute(route interface{}) *gomock.Call {
// 	mr.mock.ctrl.T.Helper()
// 	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Subroute", reflect.TypeOf((*MockRouter)(nil).Subroute), route)
// }

// // Use mocks base method.
// func (m *MockRouter) Use(arg0 rmb.Middleware) {
// 	m.ctrl.T.Helper()
// 	m.ctrl.Call(m, "Use", arg0)
// }

// // Use indicates an expected call of Use.
// func (mr *MockRouterMockRecorder) Use(arg0 interface{}) *gomock.Call {
// 	mr.mock.ctrl.T.Helper()
// 	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Use", reflect.TypeOf((*MockRouter)(nil).Use), arg0)
// }

// // WithHandler mocks base method.
// func (m *MockRouter) WithHandler(route string, handler rmb.Handler) {
// 	m.ctrl.T.Helper()
// 	m.ctrl.Call(m, "WithHandler", route, handler)
// }

// // WithHandler indicates an expected call of WithHandler.
// func (mr *MockRouterMockRecorder) WithHandler(route, handler interface{}) *gomock.Call {
// 	mr.mock.ctrl.T.Helper()
// 	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WithHandler", reflect.TypeOf((*MockRouter)(nil).WithHandler), route, handler)
// }

// // MockClient is a mock of Client interface.
// type MockClient struct {
// 	ctrl     *gomock.Controller
// 	recorder *MockClientMockRecorder
// }

// // MockClientMockRecorder is the mock recorder for MockClient.
// type MockClientMockRecorder struct {
// 	mock *MockClient
// }

// // NewMockClient creates a new mock instance.
// func NewMockClient(ctrl *gomock.Controller) *MockClient {
// 	mock := &MockClient{ctrl: ctrl}
// 	mock.recorder = &MockClientMockRecorder{mock}
// 	return mock
// }

// // EXPECT returns an object that allows the caller to indicate expected use.
// func (m *MockClient) EXPECT() *MockClientMockRecorder {
// 	return m.recorder
// }

// // Call mocks base method.
// func (m *MockClient) Call(ctx context.Context, twin uint32, fn string, data, result interface{}) error {
// 	m.ctrl.T.Helper()
// 	ret := m.ctrl.Call(m, "Call", ctx, twin, fn, data, result)
// 	ret0, _ := ret[0].(error)
// 	return ret0
// }

// // Call indicates an expected call of Call.
// func (mr *MockClientMockRecorder) Call(ctx, twin, fn, data, result interface{}) *gomock.Call {
// 	mr.mock.ctrl.T.Helper()
// 	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Call", reflect.TypeOf((*MockClient)(nil).Call), ctx, twin, fn, data, result)
// }

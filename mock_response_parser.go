// Code generated by MockGen. DO NOT EDIT.
// Source: source.go
//
// Generated by this command:
//
//	mockgen -destination=mock_response_parser.go -source=source.go -package=http -mock_names=responseParser=MockResponseParser . responseParser
//

// Package http is a generated GoMock package.
package http

import (
	context "context"
	reflect "reflect"

	opencdc "github.com/conduitio/conduit-commons/opencdc"
	gomock "go.uber.org/mock/gomock"
)

// MockrequestBuilder is a mock of requestBuilder interface.
type MockrequestBuilder struct {
	ctrl     *gomock.Controller
	recorder *MockrequestBuilderMockRecorder
	isgomock struct{}
}

// MockrequestBuilderMockRecorder is the mock recorder for MockrequestBuilder.
type MockrequestBuilderMockRecorder struct {
	mock *MockrequestBuilder
}

// NewMockrequestBuilder creates a new mock instance.
func NewMockrequestBuilder(ctrl *gomock.Controller) *MockrequestBuilder {
	mock := &MockrequestBuilder{ctrl: ctrl}
	mock.recorder = &MockrequestBuilderMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockrequestBuilder) EXPECT() *MockrequestBuilderMockRecorder {
	return m.recorder
}

// build mocks base method.
func (m *MockrequestBuilder) build(ctx context.Context, previousResponseData map[string]any, position opencdc.Position) (*Request, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "build", ctx, previousResponseData, position)
	ret0, _ := ret[0].(*Request)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// build indicates an expected call of build.
func (mr *MockrequestBuilderMockRecorder) build(ctx, previousResponseData, position any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "build", reflect.TypeOf((*MockrequestBuilder)(nil).build), ctx, previousResponseData, position)
}

// MockResponseParser is a mock of responseParser interface.
type MockResponseParser struct {
	ctrl     *gomock.Controller
	recorder *MockResponseParserMockRecorder
	isgomock struct{}
}

// MockResponseParserMockRecorder is the mock recorder for MockResponseParser.
type MockResponseParserMockRecorder struct {
	mock *MockResponseParser
}

// NewMockResponseParser creates a new mock instance.
func NewMockResponseParser(ctrl *gomock.Controller) *MockResponseParser {
	mock := &MockResponseParser{ctrl: ctrl}
	mock.recorder = &MockResponseParserMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockResponseParser) EXPECT() *MockResponseParserMockRecorder {
	return m.recorder
}

// parse mocks base method.
func (m *MockResponseParser) parse(ctx context.Context, responseBytes []byte) (*Response, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "parse", ctx, responseBytes)
	ret0, _ := ret[0].(*Response)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// parse indicates an expected call of parse.
func (mr *MockResponseParserMockRecorder) parse(ctx, responseBytes any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "parse", reflect.TypeOf((*MockResponseParser)(nil).parse), ctx, responseBytes)
}

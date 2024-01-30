// Code generated by MockGen. DO NOT EDIT.
// Source: ./dependency_capture.go
//
// Generated by this command:
//
//	mockgen -source=./dependency_capture.go --destination=./dependency_capture_mock_test.go --package=operational_eval
//

// Package operational_eval is a generated GoMock package.
package operational_eval

import (
	reflect "reflect"

	construct2 "github.com/klothoplatform/klotho/pkg/construct2"
	knowledgebase2 "github.com/klothoplatform/klotho/pkg/knowledge_base2"
	gomock "go.uber.org/mock/gomock"
)

// MockdependencyCapturer is a mock of dependencyCapturer interface.
type MockdependencyCapturer struct {
	ctrl     *gomock.Controller
	recorder *MockdependencyCapturerMockRecorder
}

// MockdependencyCapturerMockRecorder is the mock recorder for MockdependencyCapturer.
type MockdependencyCapturerMockRecorder struct {
	mock *MockdependencyCapturer
}

// NewMockdependencyCapturer creates a new mock instance.
func NewMockdependencyCapturer(ctrl *gomock.Controller) *MockdependencyCapturer {
	mock := &MockdependencyCapturer{ctrl: ctrl}
	mock.recorder = &MockdependencyCapturerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockdependencyCapturer) EXPECT() *MockdependencyCapturerMockRecorder {
	return m.recorder
}

// DAG mocks base method.
func (m *MockdependencyCapturer) DAG() construct2.Graph {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DAG")
	ret0, _ := ret[0].(construct2.Graph)
	return ret0
}

// DAG indicates an expected call of DAG.
func (mr *MockdependencyCapturerMockRecorder) DAG() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DAG", reflect.TypeOf((*MockdependencyCapturer)(nil).DAG))
}

// ExecuteDecode mocks base method.
func (m *MockdependencyCapturer) ExecuteDecode(tmpl string, data knowledgebase2.DynamicValueData, value any) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ExecuteDecode", tmpl, data, value)
	ret0, _ := ret[0].(error)
	return ret0
}

// ExecuteDecode indicates an expected call of ExecuteDecode.
func (mr *MockdependencyCapturerMockRecorder) ExecuteDecode(tmpl, data, value any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ExecuteDecode", reflect.TypeOf((*MockdependencyCapturer)(nil).ExecuteDecode), tmpl, data, value)
}

// ExecuteOpRule mocks base method.
func (m *MockdependencyCapturer) ExecuteOpRule(data knowledgebase2.DynamicValueData, rule knowledgebase2.OperationalRule) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ExecuteOpRule", data, rule)
	ret0, _ := ret[0].(error)
	return ret0
}

// ExecuteOpRule indicates an expected call of ExecuteOpRule.
func (mr *MockdependencyCapturerMockRecorder) ExecuteOpRule(data, rule any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ExecuteOpRule", reflect.TypeOf((*MockdependencyCapturer)(nil).ExecuteOpRule), data, rule)
}

// ExecutePropertyRule mocks base method.
func (m *MockdependencyCapturer) ExecutePropertyRule(data knowledgebase2.DynamicValueData, rule knowledgebase2.PropertyRule) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ExecutePropertyRule", data, rule)
	ret0, _ := ret[0].(error)
	return ret0
}

// ExecutePropertyRule indicates an expected call of ExecutePropertyRule.
func (mr *MockdependencyCapturerMockRecorder) ExecutePropertyRule(data, rule any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ExecutePropertyRule", reflect.TypeOf((*MockdependencyCapturer)(nil).ExecutePropertyRule), data, rule)
}

// GetChanges mocks base method.
func (m *MockdependencyCapturer) GetChanges() graphChanges {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetChanges")
	ret0, _ := ret[0].(graphChanges)
	return ret0
}

// GetChanges indicates an expected call of GetChanges.
func (mr *MockdependencyCapturerMockRecorder) GetChanges() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetChanges", reflect.TypeOf((*MockdependencyCapturer)(nil).GetChanges))
}

// KB mocks base method.
func (m *MockdependencyCapturer) KB() knowledgebase2.TemplateKB {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "KB")
	ret0, _ := ret[0].(knowledgebase2.TemplateKB)
	return ret0
}

// KB indicates an expected call of KB.
func (mr *MockdependencyCapturerMockRecorder) KB() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "KB", reflect.TypeOf((*MockdependencyCapturer)(nil).KB))
}
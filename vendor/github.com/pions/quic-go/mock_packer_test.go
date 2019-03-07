// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/pions/quic-go (interfaces: Packer)

// Package quic is a generated GoMock package.
package quic

import (
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	ackhandler "github.com/pions/quic-go/internal/ackhandler"
	handshake "github.com/pions/quic-go/internal/handshake"
	protocol "github.com/pions/quic-go/internal/protocol"
	wire "github.com/pions/quic-go/internal/wire"
)

// MockPacker is a mock of Packer interface
type MockPacker struct {
	ctrl     *gomock.Controller
	recorder *MockPackerMockRecorder
}

// MockPackerMockRecorder is the mock recorder for MockPacker
type MockPackerMockRecorder struct {
	mock *MockPacker
}

// NewMockPacker creates a new mock instance
func NewMockPacker(ctrl *gomock.Controller) *MockPacker {
	mock := &MockPacker{ctrl: ctrl}
	mock.recorder = &MockPackerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockPacker) EXPECT() *MockPackerMockRecorder {
	return m.recorder
}

// ChangeDestConnectionID mocks base method
func (m *MockPacker) ChangeDestConnectionID(arg0 protocol.ConnectionID) {
	m.ctrl.Call(m, "ChangeDestConnectionID", arg0)
}

// ChangeDestConnectionID indicates an expected call of ChangeDestConnectionID
func (mr *MockPackerMockRecorder) ChangeDestConnectionID(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ChangeDestConnectionID", reflect.TypeOf((*MockPacker)(nil).ChangeDestConnectionID), arg0)
}

// HandleTransportParameters mocks base method
func (m *MockPacker) HandleTransportParameters(arg0 *handshake.TransportParameters) {
	m.ctrl.Call(m, "HandleTransportParameters", arg0)
}

// HandleTransportParameters indicates an expected call of HandleTransportParameters
func (mr *MockPackerMockRecorder) HandleTransportParameters(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "HandleTransportParameters", reflect.TypeOf((*MockPacker)(nil).HandleTransportParameters), arg0)
}

// MaybePackAckPacket mocks base method
func (m *MockPacker) MaybePackAckPacket() (*packedPacket, error) {
	ret := m.ctrl.Call(m, "MaybePackAckPacket")
	ret0, _ := ret[0].(*packedPacket)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// MaybePackAckPacket indicates an expected call of MaybePackAckPacket
func (mr *MockPackerMockRecorder) MaybePackAckPacket() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "MaybePackAckPacket", reflect.TypeOf((*MockPacker)(nil).MaybePackAckPacket))
}

// PackConnectionClose mocks base method
func (m *MockPacker) PackConnectionClose(arg0 *wire.ConnectionCloseFrame) (*packedPacket, error) {
	ret := m.ctrl.Call(m, "PackConnectionClose", arg0)
	ret0, _ := ret[0].(*packedPacket)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// PackConnectionClose indicates an expected call of PackConnectionClose
func (mr *MockPackerMockRecorder) PackConnectionClose(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PackConnectionClose", reflect.TypeOf((*MockPacker)(nil).PackConnectionClose), arg0)
}

// PackPacket mocks base method
func (m *MockPacker) PackPacket() (*packedPacket, error) {
	ret := m.ctrl.Call(m, "PackPacket")
	ret0, _ := ret[0].(*packedPacket)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// PackPacket indicates an expected call of PackPacket
func (mr *MockPackerMockRecorder) PackPacket() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PackPacket", reflect.TypeOf((*MockPacker)(nil).PackPacket))
}

// PackRetransmission mocks base method
func (m *MockPacker) PackRetransmission(arg0 *ackhandler.Packet) ([]*packedPacket, error) {
	ret := m.ctrl.Call(m, "PackRetransmission", arg0)
	ret0, _ := ret[0].([]*packedPacket)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// PackRetransmission indicates an expected call of PackRetransmission
func (mr *MockPackerMockRecorder) PackRetransmission(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PackRetransmission", reflect.TypeOf((*MockPacker)(nil).PackRetransmission), arg0)
}

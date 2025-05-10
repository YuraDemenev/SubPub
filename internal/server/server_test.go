package server

import (
	"context"
	"subpun/proto/protogen"
	"testing"

	"github.com/golang/mock/gomock"
)

type mockPubStream struct {
	ctx        context.Context
	eventSlice []*protogen.Event
	sendErr    error
	recvDone   bool
}

func (m *mockPubStream) sendEvent(event *protogen.Event) error {
	if m.sendErr != nil {
		return m.sendErr
	}
	m.eventSlice = append(m.eventSlice, event)
	return nil
}

func (m *mockPubStream) getContext() context.Context {
	return m.ctx
}

func TestPublish(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSubPub
}

package server

import (
	"context"
	"errors"
	"subpun/proto/protogen"
	"subpun/subpub"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type mockPubSubStream struct {
	ctx        context.Context
	eventSlice []*protogen.Event
	sendErr    error
}

func (m *mockPubSubStream) Send(event *protogen.Event) error {
	if m.sendErr != nil {
		return m.sendErr
	}
	m.eventSlice = append(m.eventSlice, event)
	return nil
}

func (m *mockPubSubStream) SetHeader(_ metadata.MD) error {
	return errors.New("SendHeader not implemented")
}

func (m *mockPubSubStream) SendHeader(_ metadata.MD) error {
	return errors.New("SendHeader not implemented")
}

func (m *mockPubSubStream) SetTrailer(_ metadata.MD) {
}

func (m *mockPubSubStream) SendMsg(_ interface{}) error {
	return errors.New("SendMsg not implemented")
}

func (m *mockPubSubStream) RecvMsg(_ interface{}) error {
	return errors.New("RecvMsg not implemented")
}

func (m *mockPubSubStream) Context() context.Context {
	return m.ctx
}

func TestPublish(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSubPub := subpub.NewMockSubPub(ctrl)
	logger := logrus.New()
	server := &Server{
		logger: logger,
		subPub: mockSubPub,
	}

	tests := []struct {
		name           string
		req            *protogen.PublishRequest
		ctx            context.Context
		setupMock      func()
		expectedErr    error
		expectedResult *emptypb.Empty
	}{
		{
			name: "Success",
			req:  &protogen.PublishRequest{Key: "test", Data: "hello"},
			ctx:  context.Background(),
			setupMock: func() {
				mockSubPub.EXPECT().Publish("test", "hello").Return(nil)
			},
			expectedResult: &emptypb.Empty{},
		}, {
			name: "Empty Key",
			req:  &protogen.PublishRequest{Key: "", Data: "hello"},
			ctx:  context.Background(),
			setupMock: func() {
				// No Publish call expected
			},
			expectedErr: status.Error(codes.InvalidArgument, "key cannot be empty"),
		},
		{
			name: "Empty Data",
			req:  &protogen.PublishRequest{Key: "test", Data: ""},
			ctx:  context.Background(),
			setupMock: func() {
				// No Publish call expected
			},
			expectedErr: status.Error(codes.InvalidArgument, "data cannot be empty"),
		},
		{
			name: "Cancelled Context",
			req:  &protogen.PublishRequest{Key: "test", Data: "hello"},
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			}(),
			setupMock: func() {
				// No Publish call expected
			},
			expectedErr: status.Error(codes.Canceled, "request cancelled"),
		},
		{
			name: "Publish Error",
			req:  &protogen.PublishRequest{Key: "test", Data: "hello"},
			ctx:  context.Background(),
			setupMock: func() {
				mockSubPub.EXPECT().Publish("test", "hello").Return(errors.New("publish failed"))
			},
			expectedErr: status.Error(codes.Internal, "failed to publish: publish failed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock()
			result, err := server.Publish(tt.ctx, tt.req)
			if tt.expectedErr != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedErr.Error(), err.Error())
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedResult, result)
			}
		})
	}

}

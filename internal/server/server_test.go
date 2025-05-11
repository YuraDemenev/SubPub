package server

import (
	"context"
	"errors"
	"fmt"
	"subpun/internal/app"
	"subpun/internal/config"
	"subpun/proto/protogen"
	"subpun/subpub"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
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

func TestSubscribe(t *testing.T) {
	cfg, err := config.LoadConfig()
	assert.NoError(t, err, "config should load successfully")

	app := app.NewApp(cfg)
	srv := New(app)

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Run server in a goroutine
	go func() {
		err := srv.Run(cfg.GRPC.Port, ctx)
		if err != nil && err != context.Canceled {
			assert.NoError(t, err, "server should start")
		}
	}()

	// Wait for server to start
	time.Sleep(500 * time.Millisecond)
	addr := fmt.Sprintf(":%d", cfg.GRPC.Port)
	// Create gRPC client
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	assert.NoError(t, err, "client should start")
	defer conn.Close()

	client := protogen.NewPubSubClient(conn)

	// Subscribe to topic
	received := make(chan string, 1)
	subscribeErr := make(chan error, 1)

	stream, err := client.Subscribe(ctx, &protogen.SubscribeRequest{Key: "test"})
	assert.NoError(t, err, "client should subscribe")

	go func() {
		for {
			event, err := stream.Recv()
			if err != nil {
				subscribeErr <- fmt.Errorf("subscribe failed: %v", err)
				return
			}
			received <- event.Data
		}
	}()

	// Check for subscription errors
	select {
	case err := <-subscribeErr:
		assert.NoError(t, err, "subscribe shouldn`t get errors")
		return
	default:
	}

	// Publish a message
	_, err = client.Publish(ctx, &protogen.PublishRequest{Key: "test", Data: "hello"})
	assert.NoError(t, err, "publish shouldn`t fail")

	// Main loop for handling events
	for {
		select {
		case err := <-subscribeErr:
			assert.NoError(t, err, "subscribe shouldn`t get errors")
			return
		case msg := <-received:
			app.Logger.Infof("Received message: %s", msg)
			assert.Equal(t, "hello", msg, "Expected message 'hello'")
			return
		default:
			// Non-blocking check for stream message
			event, err := stream.Recv()
			if err != nil {
				subscribeErr <- fmt.Errorf("stream receive failed: %v", err)
				continue
			}
			received <- event.Data
		}
	}
}

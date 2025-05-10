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

// func main() {
// 	cfg, err := config.LoadConfig()
// 	if err != nil {
// 		log.Fatalf("Failed to load config: %v", err)
// 	}

// 	app := app.NewApp(cfg)
// 	//Log info
// 	app.Logger.Info("Application started")
// 	app.Logger.Infof("gRPC Port: %d", cfg.GRPC.Port)
// 	app.Logger.Infof("Logging Level: %s", cfg.Logging.Level)

// 	srv := server.New(app)

// 	// Create context for graceful shutdown
// 	ctx, cancel := context.WithCancel(context.Background())
// 	defer cancel()

// 	// Handle OS signals for graceful shutdown
// 	sigChan := make(chan os.Signal, 1)
// 	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
// 	// Run server in a goroutine
// 	go func() {
// 		if err := srv.Run(cfg.GRPC.Port, ctx); err != nil && err != context.Canceled {
// 			app.Logger.Errorf("Server failed: %v", err)
// 		}
// 	}()

// 	// Wait for server to start
// 	time.Sleep(500 * time.Millisecond)
// 	addr := fmt.Sprintf(":%d", cfg.GRPC.Port)
// 	// Create gRPC client
// 	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
// 	if err != nil {
// 		app.Logger.Errorf("Failed to dial server: %v", err)
// 		log.Fatalf("Failed to dial server: %v", err)
// 	}
// 	defer conn.Close()

// 	client := protogen.NewPubSubClient(conn)
// 	app.Logger.Info("gRPC client connected")

// 	// Subscribe to topic
// 	received := make(chan string, 1)
// 	subscribeErr := make(chan error, 1)

// 	stream, err := client.Subscribe(ctx, &protogen.SubscribeRequest{Key: "test"})
// 	if err != nil {
// 		return
// 	}
// 	app.Logger.Info("Subscribed to topic: test")

// 	go func() {
// 		for {
// 			event, err := stream.Recv()
// 			if err != nil {
// 				subscribeErr <- fmt.Errorf("subscribe failed: %v", err)
// 				return
// 			}
// 			received <- event.Data
// 		}
// 	}()

// 	// Check for subscription errors
// 	select {
// 	case err := <-subscribeErr:
// 		app.Logger.Errorf("Subscription error: %v", err)
// 		log.Fatalf("Subscription error: %v", err)
// 	default:
// 	}

// 	// Publish a message
// 	_, err = client.Publish(ctx, &protogen.PublishRequest{Key: "test", Data: "hello"})
// 	if err != nil {
// 		app.Logger.Errorf("Publish failed: %v", err)
// 		log.Fatalf("Publish failed: %v", err)
// 	}
// 	app.Logger.Info("Published message: hello")

// 	// Main loop for handling events
// 	for {
// 		select {

// 		case err := <-subscribeErr:
// 			app.Logger.Errorf("Subscription error: %v", err)
// 			log.Fatalf("Subscription error: %v", err)

// 		case msg := <-received:
// 			app.Logger.Infof("Received message: %s", msg)
// 			if msg != "hello" {
// 				app.Logger.Errorf("Expected message 'hello', got '%s'", msg)
// 				log.Fatalf("Expected message 'hello', got '%s'", msg)
// 			}
// 			app.Logger.Info("Message verified, waiting for shutdown signal...")
// 			// Continue to wait for shutdown signal

// 		case sig := <-sigChan:
// 			app.Logger.Infof("Received signal: %v, stopping server...", sig)
// 			cancel()
// 			time.Sleep(1 * time.Second)
// 			app.Logger.Info("Server stopped")
// 			return

// 		default:
// 			// Non-blocking check for stream message
// 			event, err := stream.Recv()
// 			if err != nil {
// 				subscribeErr <- fmt.Errorf("stream receive failed: %v", err)
// 				continue
// 			}
// 			received <- event.Data
// 		}
// 	}
// }

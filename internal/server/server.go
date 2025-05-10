package server

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"subpun/internal/app"
	"subpun/proto/protogen"
	"subpun/subpub"
	"syscall"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// gRPC server
type Server struct {
	grpcServer *grpc.Server
	logger     *logrus.Logger
	subPub     subpub.SubPub
	protogen.UnimplementedPubSubServer
}

// Create new gRPC server
func New(app *app.App) *Server {
	s := &Server{
		logger: app.Logger,
		subPub: app.SubPub,
	}
	s.grpcServer = grpc.NewServer()
	protogen.RegisterPubSubServer(s.grpcServer, s)
	return s
}

// Run gRPC server
func (s *Server) Run(port int) error {
	addr := fmt.Sprintf(":%d", port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("Failed to listen: %s", err.Error())
	}

	// Chan for OC sygnal
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	// Context with graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())

	// Run server
	go func() {
		s.logger.Infof("Starting gRPC server on %s", addr)
		if err := s.grpcServer.Serve(lis); err != nil {
			s.logger.WithError(err).Fatal("failed to serve")
		}
	}()

	// Wait for stop sygnal
	<-stop
	s.logger.Info("Received shutdon signal, initiating shutdown")
	// Cancel context for stop subPub
	cancel()

	// Call close for subPub
	if err := s.subPub.Close(ctx); err != nil {
		s.logger.WithError(err).Error("Failed to close subPub")
	} else {
		s.logger.Info("subPub closed successfully")
	}

	//Stop gRPC server
	s.grpcServer.GracefulStop()
	s.logger.Info("gRPC server stopped")
	return nil
}

func (s *Server) Subscribe(req *protogen.SubscribeRequest, stream protogen.PubSub_SubscribeServer) error {
	s.logger.WithField("key", req.Key).Info("New subscriptiob request")

	eventCh := make(chan *protogen.Event, 100)

	// Create the callback
	callback := func(msg interface{}) {
		message, ok := msg.(subpub.Message)
		if !ok {
			s.logger.Warnf("received unknown message type message:%v", msg)
			return
		}
		dataStr, ok := message.Data.(string)
		if !ok {
			s.logger.Warnf("received non-string data in message data:%v", message.Data)
			return
		}

		event := &protogen.Event{Data: dataStr}
		select {
		case eventCh <- event:
			s.logger.WithFields(logrus.Fields{
				"key":  req.Key,
				"data": dataStr,
			}).Info("Event sent to channel")
		case <-stream.Context().Done():
			s.logger.WithField("key", req.Key).Info("Stream context done")
		}
	}

	// Subscribe
	sub, err := s.subPub.Subscribe(req.Key, callback)
	defer sub.Unsubscribe()

	if err != nil {
		s.logger.WithError(err).Error("Failed to subscribe")
		return fmt.Errorf("Failed to subscribe: %w", err)
	}

	for {
		select {
		case event := <-eventCh:
			//Send message to gRPC stream
			if err := stream.Send(event); err != nil {
				s.logger.WithError(err).Error("Failed to send event")
				return status.Errorf(codes.Internal, "Failed to send event: %s", err.Error())
			}

			s.logger.WithFields(logrus.Fields{
				"key":  req.Key,
				"data": event.Data,
			}).Info("Event send to client")

		case <-stream.Context().Done():
			s.logger.WithField("key", req.Key).Info("Client desconnected")
			return stream.Context().Err()
		default:
			s.logger.WithField("key", req.Key).Warn("Message dropped: channel full")
		}
	}
}

func (s *Server) Publish(ctx context.Context, req *protogen.PublishRequest) (*emptypb.Empty, error) {
	// Checks
	if req.Key == "" {
		s.logger.Error("Publish request has an empty key")
		return nil, status.Errorf(codes.InvalidArgument, "key cannot be empty")
	}
	if req.Data == "" {
		s.logger.WithField("key", req.Key).Error("Publish request with empty data")
		return nil, status.Errorf(codes.InvalidArgument, "data cannot be empty")
	}

	// log request
	s.logger.WithFields(logrus.Fields{
		"key":  req.Key,
		"data": req.Data,
	}).Info("Request for publish")

	select {
	case <-ctx.Done():
		s.logger.WithFields(logrus.Fields{
			"key":  req.Key,
			"data": req.Data,
		}).Info("Publish request cancelled")
		return nil, status.Error(codes.Canceled, "Request cancelled")

	default:
		// Publish message
		if err := s.subPub.Publish(req.Key, req.Data); err != nil {
			s.logger.WithFields(logrus.Fields{
				"key":  req.Key,
				"data": req.Data,
			}).WithError(err).Error("Failed to publish")
			return nil, status.Errorf(codes.Internal, "Failed to publish: %s", err.Error())
		}

		// Publish success
		s.logger.WithFields(logrus.Fields{
			"key":  req.Key,
			"data": req.Data,
		}).Info("Message published successfully")
		return &emptypb.Empty{}, nil
	}
}

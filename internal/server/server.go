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

	//Chan for OC sygnal
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

	// Wait stop sygnal
	<-stop
	s.logger.Info("Received shutdon signal, initiating shutdown")
	// Cancel context for stop subPub
	cancel()

	// Call close for subPub
	if err := s.subPub.Close(ctx); err != nil {
		s.logger.WithError(err).Error("Failed to close subPub")
	}

	//Stop gRPC server
	s.grpcServer.GracefulStop()
	s.logger.Info("gRPC server stopped")
	return nil
}

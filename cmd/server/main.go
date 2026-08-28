package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"

	ordersv1 "github.com/vlantonov/GoGrpcKafkaMicroservice/gen/orders/v1"
	"github.com/vlantonov/GoGrpcKafkaMicroservice/internal/config"
	"github.com/vlantonov/GoGrpcKafkaMicroservice/internal/grpc/handler"
	"github.com/vlantonov/GoGrpcKafkaMicroservice/internal/health"
	"github.com/vlantonov/GoGrpcKafkaMicroservice/internal/kafka/consumer"
	"github.com/vlantonov/GoGrpcKafkaMicroservice/internal/kafka/producer"
	"github.com/vlantonov/GoGrpcKafkaMicroservice/internal/store"
)

// EventConsumer is the background consumer managed by the process lifecycle.
type EventConsumer interface {
	Start(ctx context.Context) error
	Close() error
}

func main() {
	cfg := config.Load()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	memStore := store.NewMemoryStore()

	prod, err := producer.NewSaramaProducer(cfg.KafkaBrokers, cfg.KafkaTopic)
	if err != nil {
		logger.Error("failed to create kafka producer", slog.Any("error", err))
		os.Exit(1)
	}

	cons, err := consumer.NewSaramaConsumer(cfg.KafkaBrokers, cfg.KafkaGroupID, cfg.KafkaTopic, logger)
	if err != nil {
		logger.Error("failed to create kafka consumer", slog.Any("error", err))
		os.Exit(1)
	}

	orderHandler := handler.NewOrderServiceServer(memStore, prod, logger)

	grpcServer := grpc.NewServer()
	ordersv1.RegisterOrderServiceServer(grpcServer, orderHandler)

	lis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		logger.Error("failed to listen", slog.Any("error", err))
		os.Exit(1)
	}

	healthSrv := health.NewServer(cfg.HealthPort)

	// Start consumer goroutine.
	consumerErrCh := make(chan error, 1)
	go func() {
		if err := cons.Start(ctx); err != nil {
			consumerErrCh <- err
		}
		close(consumerErrCh)
	}()

	// Start health server goroutine.
	go func() {
		logger.Info("health server listening", slog.String("port", cfg.HealthPort))
		if err := healthSrv.ListenAndServe(); err != nil {
			logger.Info("health server stopped", slog.Any("reason", err))
		}
	}()

	// Start gRPC server goroutine.
	go func() {
		logger.Info("gRPC server listening", slog.String("port", cfg.GRPCPort))
		if err := grpcServer.Serve(lis); err != nil {
			logger.Error("gRPC server error", slog.Any("error", err))
		}
	}()

	// Block until signal.
	<-ctx.Done()
	logger.Info("shutdown signal received")

	// Graceful shutdown sequence.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	grpcDone := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(grpcDone)
	}()
	select {
	case <-grpcDone:
	case <-shutdownCtx.Done():
		grpcServer.Stop()
	}

	if err := cons.Close(); err != nil {
		logger.Error("consumer close error", slog.Any("error", err))
	}
	if err := prod.Close(); err != nil {
		logger.Error("producer close error", slog.Any("error", err))
	}
	if err := healthSrv.Shutdown(shutdownCtx); err != nil {
		logger.Error("health server shutdown error", slog.Any("error", err))
	}

	if err := <-consumerErrCh; err != nil {
		fmt.Fprintf(os.Stderr, "consumer error: %v\n", err)
	}

	logger.Info("shutdown complete")
}

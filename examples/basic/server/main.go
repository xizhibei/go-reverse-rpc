package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-playground/validator/v10"
	rrpc "github.com/xizhibei/go-reverse-rpc"
	"github.com/xizhibei/go-reverse-rpc/examples/basic"
	"github.com/xizhibei/go-reverse-rpc/mqttadapter"
	"github.com/xizhibei/go-reverse-rpc/mqttjson"
	"github.com/xizhibei/go-reverse-rpc/telemetry"
	"go.uber.org/zap"
)

func main() {
	brokerURL := flag.String("broker", "tcp://localhost:1883", "MQTT broker URL")
	clientID := flag.String("client-id", "math-server", "MQTT client ID")
	flag.Parse()

	logger, _ := zap.NewDevelopment()
	zap.ReplaceGlobals(logger)
	defer logger.Sync()

	log := logger.Sugar()

	// Create MQTT client
	mqttClient, err := mqttadapter.New(*brokerURL, *clientID)
	if err != nil {
		log.Fatalf("Failed to create MQTT client: %v", err)
	}

	// Create server
	server := mqttjson.NewServer(
		mqttClient,
		"math",      // topic prefix
		"server-01", // device ID
		validator.New(),
	)
	log.Infof("Server connected to mqtt broker %s", *brokerURL)

	traceFile, err := os.OpenFile("server-trace.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Fatalf("Failed to open trace file: %v", err)
	}
	defer traceFile.Close()

	metricFile, err := os.OpenFile("server-metric.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Fatalf("Failed to open metric file: %v", err)
	}
	defer metricFile.Close()

	// Initialize telemetry
	tel, err := telemetry.New(context.Background(), telemetry.Config{
		ServiceName:    "math-server",
		ServiceVersion: "1.0.0",
		Environment:    "development",
		Debug:          true,
		TraceWriter:    traceFile,
		MetricWriter:   metricFile,
	})
	if err != nil {
		log.Infof("Warning: Failed to initialize telemetry: %v", err)
	}
	server.SetTelemetry(tel)

	// Register math operations
	registerMathOperations(server)
	log.Infof("Registered math operations: add, subtract, multiply, divide")

	// Handle graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Infof("Received signal %v, shutting down...", sig)
		cancel()
	}()

	<-ctx.Done()
}

func registerMathOperations(server *mqttjson.Server) {
	// Addition
	server.Register("add", &rrpc.Handler{
		Method: func(c rrpc.Context) {
			var req basic.MathRequest
			if err := c.Bind(&req); err != nil {
				c.ReplyError(rrpc.RPCStatusClientError, fmt.Errorf("invalid request: %w", err))
				return
			}

			result := basic.MathResponse{Result: req.A + req.B}
			c.ReplyOK(result)
		},
		Timeout: 5 * time.Second,
	})

	// Subtraction
	server.Register("subtract", &rrpc.Handler{
		Method: func(c rrpc.Context) {
			var req basic.MathRequest
			if err := c.Bind(&req); err != nil {
				c.ReplyError(rrpc.RPCStatusClientError, fmt.Errorf("invalid request: %w", err))
				return
			}

			result := basic.MathResponse{Result: req.A - req.B}
			c.ReplyOK(result)
		},
		Timeout: 5 * time.Second,
	})

	// Multiplication
	server.Register("multiply", &rrpc.Handler{
		Method: func(c rrpc.Context) {
			var req basic.MathRequest
			if err := c.Bind(&req); err != nil {
				c.ReplyError(rrpc.RPCStatusClientError, fmt.Errorf("invalid request: %w", err))
				return
			}

			result := basic.MathResponse{Result: req.A * req.B}
			c.ReplyOK(result)
		},
		Timeout: 5 * time.Second,
	})

	// Division
	server.Register("divide", &rrpc.Handler{
		Method: func(c rrpc.Context) {
			var req basic.MathRequest
			if err := c.Bind(&req); err != nil {
				c.ReplyError(rrpc.RPCStatusClientError, fmt.Errorf("invalid request: %w", err))
				return
			}

			if req.B == 0 {
				c.ReplyError(rrpc.RPCStatusClientError, fmt.Errorf("division by zero"))
				return
			}

			result := basic.MathResponse{Result: req.A / req.B}
			c.ReplyOK(result)
		},
		Timeout: 5 * time.Second,
	})
}

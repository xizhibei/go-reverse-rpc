package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/xizhibei/go-reverse-rpc/examples/basic"
	"github.com/xizhibei/go-reverse-rpc/mqttadapter"
	"github.com/xizhibei/go-reverse-rpc/mqttjson"
	"github.com/xizhibei/go-reverse-rpc/telemetry"
	"go.uber.org/zap"
)

func main() {
	brokerURL := flag.String("broker", "tcp://localhost:1883", "MQTT broker URL")
	clientID := flag.String("client-id", "math-client", "MQTT client ID")
	serverID := flag.String("server-id", "server-01", "Server device ID")
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

	// Create RPC client
	client := mqttjson.NewClient(
		mqttClient,
		"math", // topic prefix
	)
	log.Infof("Client connected to mqtt broker %s", *brokerURL)

	traceFile, err := os.OpenFile("client-trace.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Fatalf("Failed to open trace file: %v", err)
	}
	defer traceFile.Close()

	metricFile, err := os.OpenFile("client-metric.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Fatalf("Failed to open metric file: %v", err)
	}
	defer metricFile.Close()

	// Initialize telemetry (now optional via environment variables)
	tel, err := telemetry.New(context.Background(), telemetry.Config{
		ServiceName:    "math-client",
		ServiceVersion: "1.0.0",
		Environment:    "development",
		Debug:          true,
		Enabled:        true, // Enable by default for demo
		TraceWriter:    traceFile,
		MetricWriter:   metricFile,
	})
	if err != nil {
		log.Warnf("Failed to initialize telemetry: %v", err)
		// Use no-op telemetry as fallback
		tel, _ = telemetry.NewNoop()
	}
	client.SetTelemetry(tel)

	// Create context with cancellation for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Infof("Received signal %v, shutting down...", sig)
		cancel()
	}()

	// Run example operations
	go func() {
		// Wait for connection to establish
		time.Sleep(1 * time.Second)

		// Example 1: Addition
		addReq := basic.MathRequest{A: 5, B: 3}
		var addRes basic.MathResponse
		err := client.Call(ctx, *serverID, "add", &addReq, &addRes)
		if err != nil {
			log.Infof("Addition error: %v", err)
		} else {
			log.Infof("Add result: %.2f + %.2f = %.2f", addReq.A, addReq.B, addRes.Result)
		}

		// Example 2: Multiplication
		mulReq := basic.MathRequest{A: 4, B: 6}
		var mulRes basic.MathResponse
		err = client.Call(ctx, *serverID, "multiply", &mulReq, &mulRes)
		if err != nil {
			log.Infof("Multiplication error: %v", err)
		} else {
			log.Infof("Multiply result: %.2f * %.2f = %.2f", mulReq.A, mulReq.B, mulRes.Result)
		}

		// Example 3: Division by zero (error handling)
		divReq := basic.MathRequest{A: 10, B: 0}
		var divRes basic.MathResponse
		err = client.Call(ctx, *serverID, "divide", &divReq, &divRes)
		if err != nil {
			log.Infof("Division error (expected): %v", err)
		} else {
			log.Infof("Divide result: %.2f / %.2f = %.2f", divReq.A, divReq.B, divRes.Result)
		}

		// Example 4: Subtraction with timeout
		timeoutCtx, timeoutCancel := context.WithTimeout(ctx, 100*time.Millisecond)
		defer timeoutCancel()

		subReq := basic.MathRequest{A: 10, B: 3}
		var subRes basic.MathResponse
		err = client.Call(timeoutCtx, *serverID, "subtract", &subReq, &subRes)
		if err != nil {
			log.Infof("Subtraction error: %v", err)
		} else {
			log.Infof("Subtract result: %.2f - %.2f = %.2f", subReq.A, subReq.B, subRes.Result)
		}

		// Signal completion
		cancel()
	}()

	// Wait for completion or interruption
	<-ctx.Done()

	// Wait for any key to exit
	log.Infof("Press any key to exit...")
	fmt.Scanln()

	log.Infof("Client shutting down...")
}

# Go reverse RPC

A remote procedure call (RPC) framework designed for connecting to devices remotely. It enables the "server" to call functions provided by the "client". This inverted model is particularly useful for devices behind firewalls or NATs that can't accept incoming connections.

[![Build Status](https://github.com/xizhibei/go-reverse-rpc/actions/workflows/go.yml/badge.svg)](https://github.com/xizhibei/go-reverse-rpc/actions/workflows/go.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/xizhibei/go-reverse-rpc)](https://goreportcard.com/report/github.com/xizhibei/go-reverse-rpc)
[![GoDoc](https://pkg.go.dev/badge/github.com/xizhibei/go-reverse-rpc?status.svg)](https://pkg.go.dev/github.com/xizhibei/go-reverse-rpc?tab=doc)
<!-- [![codecov](https://codecov.io/gh/xizhibei/go-reverse-rpc/branch/master/graph/badge.svg)](https://codecov.io/gh/xizhibei/go-reverse-rpc) -->
<!-- [![Sourcegraph](https://sourcegraph.com/github.com/xizhibei/go-reverse-rpc/-/badge.svg)](https://sourcegraph.com/github.com/xizhibei/go-reverse-rpc?badge) -->
<!-- [![Release](https://img.shields.io/github/release/xizhibei/go-reverse-rpc.svg?style=flat-square)](https://github.com/xizhibei/go-reverse-rpc/releases) -->

## Architecture Overview

In traditional RPC, clients connect to servers. In reverse RPC, both the server (service provider) and client (service consumer) connect to a message broker:

```
Traditional RPC: Client → Server
Reverse RPC:    Server → Broker ← Client
```

This architecture is ideal for:
- Services behind firewalls/NATs
- Dynamic service registration
- Centralized routing control

## Features

- **Multiple Transport Protocols**
  - MQTT 3.1/3.11 (implemented)
  - MQTT v5, WebSocket, AMQP (planned)
- **Flexible Data Formats**
  - JSON and Protobuf (implemented)
  - MessagePack, Avro (planned)
- **Observability**
  - OpenTelemetry integration (tracing and metrics)
  - Configurable telemetry export (OTLP, debug mode)
  - Built-in RPC performance metrics
- **Reliability**
  - Structured error handling
  - Request rate limiting
  - Configurable timeouts
- **Performance**
  - Payload compression
  - Worker pool management

## Installation

```bash
go get github.com/xizhibei/go-reverse-rpc@latest
```

## Quick Start

### Server Setup
```go
import (
    "github.com/xizhibei/go-reverse-rpc/mqttpb"
    "github.com/xizhibei/go-reverse-rpc/mqttadapter"
)

mqttClient, err := mqttadapter.New("tcp://localhost", "client-id-123456-server")
if err != nil {
    panic(err)
}

server := mqttpb.NewServer(
    mqttClient,
    "example-prefix",
    "device-123456",
)
```

### Client Setup
```go
import (
    "github.com/xizhibei/go-reverse-rpc/mqttpb"
    "github.com/xizhibei/go-reverse-rpc/mqttadapter"
)

mqttClient, err := mqttadapter.New("tcp://localhost", "client-id-123456-client")
if err != nil {
    panic(err)
}

client := mqttpb.New(
    mqttClient,
    "example-prefix",
    mqttpb.ContentEncoding_GZIP,
)
```

### Register Handler (Server Side)
```go
import (
    rrpc "github.com/xizhibei/go-reverse-rpc"
)

server.Register("example-method", &rrpc.Handler{
    Method: func(c rrpc.Context) {
        var req Req
        err := c.Bind(&req)
        if err != nil {
            c.ReplyError(rrpc.RPCStatusClientError, err)
            return
        }

        // your business logic ...

        c.ReplyOK(req)
    },
    Timeout: 5 * time.Second,
})
```

### Make RPC Call (Client Side)
```go
var res Req
err := client.Call(context.Background(), "device-123456", "example-method", &reqParams, &res)
```

## Telemetry Configuration

The framework includes comprehensive OpenTelemetry support with both programmatic and environment-based configuration.

### Environment Variables

```bash
# Enable/disable telemetry (default: false)
export OTEL_ENABLED=true

# Enable debug mode with stdout output (default: false)  
export OTEL_DEBUG=true

# OTLP collector endpoint (default: localhost:4317)
export OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317

# Deployment environment (default: development)
export ENVIRONMENT=production
```

### Programmatic Configuration

```go
import "github.com/xizhibei/go-reverse-rpc/telemetry"

tel, err := telemetry.New(telemetry.Config{
    ServiceName:    "my-service",
    ServiceVersion: "1.0.0", 
    Environment:    "production",
    OTLPEndpoint:   "localhost:4317",
    Debug:          false,
    Enabled:        true,
})
if err != nil {
    // Handle error
}
defer tel.Shutdown(context.Background())

// Configure server with telemetry
server.SetTelemetry(tel)
```

## Server Configuration Options

```go
// Server configuration options
rrpc.WithServerName(name string)       // Set server name for metrics labels
rrpc.WithLogResponse(logResponse bool) // Enable/disable response logging
rrpc.WithLimiter(d time.Duration, count int) // Set rate limiter parameters
rrpc.WithLimiterReject()               // Reject requests when limiter is full (default)
rrpc.WithLimiterWait()                 // Wait for resources instead of rejecting
rrpc.WithWorkerNum(count int)          // Set worker pool size
```

## Examples

The project includes comprehensive examples to help you get started:

### Basic Client/Server Demo

Located in `examples/basic/`, this example demonstrates:
- Simple server with math operations (Add, Multiply, Divide)
- Client implementation making RPC calls
- Error handling and timeout management
- OpenTelemetry integration with metrics and tracing

```bash
# Start the server
go run examples/basic/server/main.go

# In another terminal, run the client
go run examples/basic/client/main.go
```

The example includes both manual telemetry configuration and environment-based configuration patterns. Check `examples/basic/server-with-env/main.go` for environment variable usage.


## License

Go reverse RPC is released under MIT license, refer to [LICENSE](LICENSE) file.

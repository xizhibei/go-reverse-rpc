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
  - Prometheus metrics
  - OpenTelemetry integration
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

## Configuration Options

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
- Simple server with math operations
- Client implementation making RPC calls
- Error handling and timeout management
- Telemetry integration

```bash
# Start the server
go run examples/basic/server.go

# In another terminal, run the client
go run examples/basic/client.go
```

## Configuration Options

```go
rrpc.WithServerName(name string) // Used to set the name of the server. For monitoring purposes, metrics labels will use this name.
rrpc.WithLogResponse(logResponse bool) // Used to enable or disable logging of response.
rrpc.WithLimiter(d time.Duration, count int) // Used to set the limiter duration and count for the server.
rrpc.WithLimiterReject() // Used to allow the server to reject requests when the limiter is full. This is default behavior.
rrpc.WithLimiterWait() // Used to allow the server to wait for available resources instead of rejecting requests when the limiter is full.
rrpc.WithWorkerNum(count int) // Used to set the number of workers for the server.
```

## License

Go reverse RPC is released under MIT license, refer to [LICENSE](LICENSE) file.

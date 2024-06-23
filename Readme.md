# Go reverse RPC

A remote procedure call (RPC) framework designed for connecting to devices remotely. It enables the "server" to call functions provided by the "client".

[![Build Status](https://github.com/xizhibei/go-reverse-rpc/actions/workflows/go.yml/badge.svg)](https://github.com/xizhibei/go-reverse-rpc/actions/workflows/go.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/xizhibei/go-reverse-rpc)](https://goreportcard.com/report/github.com/xizhibei/go-reverse-rpc)
[![GoDoc](https://pkg.go.dev/badge/github.com/xizhibei/go-reverse-rpc?status.svg)](https://pkg.go.dev/github.com/xizhibei/go-reverse-rpc?tab=doc)
<!-- [![codecov](https://codecov.io/gh/xizhibei/go-reverse-rpc/branch/master/graph/badge.svg)](https://codecov.io/gh/xizhibei/go-reverse-rpc) -->
<!-- [![Sourcegraph](https://sourcegraph.com/github.com/xizhibei/go-reverse-rpc/-/badge.svg)](https://sourcegraph.com/github.com/xizhibei/go-reverse-rpc?badge) -->
<!-- [![Release](https://img.shields.io/github/release/xizhibei/go-reverse-rpc.svg?style=flat-square)](https://github.com/xizhibei/go-reverse-rpc/releases) -->


## Features

- Supports multiple communication protocols - currently implemented MQTT 3.1/3.11
- Allows encoding data in different formats - currently supports JSON and Protobuf
- Provides monitoring metrics for system insights
- Implements error handling mechanisms for reliability

#### TODO

- Open telemetry support
- MQTT v5 protocol support
- WebSocket protocol support
- AMQP protocol support

## Installation

```bash
go get github.com/xizhibei/go-reverse-rpc@latest
```

## Usage

#### Server create
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

#### Client create
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

#### Register handler on server side
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

#### Call on client side
```go
var res Req
err := client.Call(context.Background(), "device-123456", "example-method", &reqParams, &res)
```

#### Server create options

```go
rrpc.WithServerName(name string) // Used to set the name of the server. For monitoring purposes, metrics labels will use this name.
rrpc.WithLogResponse(logResponse bool) // Used to enable or disable logging of response.
rrpc.WithLimiter(d time.Duration, count int) // Used to set the limiter duration and count for the server.
rrpc.WithLimiterReject() // Used to allow the server to reject requests when the limiter is full. This is default behavior.
rrpc.WithLimiterWait() // Used to allow the server to wait for available resources instead of rejecting requests when the limiter is full.
rrpc.WithWorkerNum(count int) // Used to set the number of workers for the server.
```

## License

Go reverse RPC released under MIT license, refer [LICENSE](LICENSE) file.

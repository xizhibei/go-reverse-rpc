# Go reverse RPC

A remote procedure call (RPC) framework designed for connecting to devices remotely. It enables the "server" to call functions provided by the "client".

[![Build Status](https://github.com/xizhibei/go-reverse-rpc/workflows/go/badge.svg?branch=master)](https://github.com/xizhibei/go-reverse-rpc/actions?query=branch%3Amaster)
[![Go Report Card](https://goreportcard.com/badge/github.com/xizhibei/go-reverse-rpc)](https://goreportcard.com/report/github.com/xizhibei/go-reverse-rpc)
[![GoDoc](https://pkg.go.dev/badge/github.com/xizhibei/go-reverse-rpc?status.svg)](https://pkg.go.dev/github.com/xizhibei/go-reverse-rpc?tab=doc)
<!-- [![codecov](https://codecov.io/gh/xizhibei/go-reverse-rpc/branch/master/graph/badge.svg)](https://codecov.io/gh/xizhibei/go-reverse-rpc) -->
<!-- [![Sourcegraph](https://sourcegraph.com/github.com/xizhibei/go-reverse-rpc/-/badge.svg)](https://sourcegraph.com/github.com/gin-gonic/gin?badge) -->
<!-- [![Release](https://img.shields.io/github/release/xizhibei/go-reverse-rpc.svg?style=flat-square)](https://github.com/xizhibei/go-reverse-rpc/releases) -->

## WARNING

This project is currently under development, and the API may undergo breaking changes. Please use it at your own risk.


## Features

- Based on MQTT (or other long term connections, WebSocket is on the way.)
- JSON or Protobuf encoding

## Get start

```bash
go get github.com/xizhibei/go-reverse-rpc
```

## Usage

#### Server create
```go
service, err := mqtt_pb_server.New(&mqtt_pb_server.MQTTOptions{
    Uri:   "tcp://localhost",
    Qos:   0,
    Topic: path.Join("example", "123456", "request/+"),
})
if err != nil {
    panic(err)
}
```

#### Client create
```go
client, err := mqtt_pb_client.New(
    "tcp://localhost",
    "123456-client",
    "example",
)
if err != nil {
    panic(err)
}
```

#### Register handler on server side
```go
server.Register(method, &reverse_rpc.Handler{
    Method: func(c reverse_rpc.Context) {
        var req Req
        err := c.Bind(&req)
        if err != nil {
            c.ReplyError(400, err)
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
err := client.Call(context.Background(), suite.deviceId, method, &reqParams, &res)
```

## License

Go reverse RPC released under MIT license, refer [LICENSE](LICENSE) file.

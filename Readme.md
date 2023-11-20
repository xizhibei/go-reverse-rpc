# Go reverse RPC

### Intro

go-reverse-rpc is a rpc framework for remote device connection. It enable "server" call functions provided by "client"

Current it only support MQTT protocal, WebSocket is on the way.


### Features

- Based on MQTT (or other long term connections)
- JSON or Protobuf encoding

### Get start

```bash
go get github.com/xizhibei/go-reverse-rpc
```

### Examples

```go
// server side
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

// client side
var res Req
err := client.Call(context.Background(), suite.deviceId, method, &reqParams, &res)
```

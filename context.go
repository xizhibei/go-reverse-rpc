package reverse_rpc

//go:generate mockgen -source=context.go -destination=mock/mock_context.go

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/atomic"
)

type ID struct {
	Num uint64
	Str string
}

func (id *ID) String() string {
	if id.Str != "" {
		return id.Str
	}
	return strconv.FormatUint(id.Num, 10)
}

type Response struct {
	Result interface{}
	Error  error
	Status int
}

type Context interface {
	ID() *ID
	Method() string
	Context() context.Context
	ReplyDesc() string
	Bind(request interface{}) error
	Reply(res *Response) bool
	ReplyOK(data interface{}) bool
	ReplyError(status int, err error) bool
	GetResponse() *Response
	PrometheusLabels() prometheus.Labels
}

type BaseContext struct {
	res       *Response
	resMu     sync.Mutex
	replyed   atomic.Bool
	BaseReply func(res *Response)
	ctx       context.Context
}

func (c *BaseContext) Context() context.Context {
	if c.ctx == nil {
		return context.Background()
	}
	return c.ctx
}

func (c *BaseContext) Reply(res *Response) bool {
	if !c.replyed.CompareAndSwap(false, true) {
		return false
	}

	c.setResponse(res)

	c.BaseReply(res)

	return true
}

func (c *BaseContext) ReplyOK(data interface{}) bool {
	return c.Reply(&Response{
		Status: 200,
		Result: data,
	})
}

func (c *BaseContext) ReplyError(status int, err error) bool {
	return c.Reply(&Response{
		Status: status,
		Error:  err,
	})
}

func (c *BaseContext) setResponse(res *Response) {
	c.resMu.Lock()
	defer c.resMu.Unlock()
	c.res = res
}

func (c *BaseContext) GetResponse() *Response {
	c.resMu.Lock()
	defer c.resMu.Unlock()
	return c.res
}

type Handler struct {
	Method  func(c Context)
	Timeout time.Duration
}

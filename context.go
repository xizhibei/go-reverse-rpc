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

// ID represents an identifier with a numeric value and a string value.
type ID struct {
	Num uint64 // Num is the numeric value of the identifier.
	Str string // Str is the string value of the identifier.
}

// String returns the string representation of the ID.
// If the ID has a non-empty string representation, it returns the string representation.
// Otherwise, it returns the numeric representation of the ID as a decimal string.
func (id *ID) String() string {
	if id.Str != "" {
		return id.Str
	}
	return strconv.FormatUint(id.Num, 10)
}

// Response represents a response message.
// Result holds the response data.
// Error holds any error that occurred during the request.
// Status holds the status code of the response.
type Response struct {
	Result interface{}
	Error  error
	Status int
}

// Context represents the context for reverse RPC.
type Context interface {
	// ID returns the unique identifier of the context.
	ID() *ID

	// Method returns the name of the RPC method.
	Method() string

	// Context returns the underlying context.Context.
	Context() context.Context

	// ReplyDesc returns the description of the reply message.
	ReplyDesc() string

	// Bind binds the request data to the context.
	Bind(request interface{}) error

	// Reply sends a response message.
	// It returns true if the response was sent successfully, false otherwise.
	Reply(res *Response) bool

	// ReplyOK sends a successful response message with the given data.
	// It returns true if the response was sent successfully, false otherwise.
	ReplyOK(data interface{}) bool

	// ReplyError sends an error response message with the given status and error.
	// It returns true if the response was sent successfully, false otherwise.
	ReplyError(status int, err error) bool

	// GetResponse returns the response message.
	GetResponse() *Response

	// PrometheusLabels returns the Prometheus labels associated with the context.
	PrometheusLabels() prometheus.Labels
}

// BaseContext represents the base context for reverse RPC.
type BaseContext struct {
	res       *Response           // res is the response object.
	resMu     sync.Mutex          // resMu is a mutex to synchronize access to the response object.
	replyed   atomic.Bool         // replyed is an atomic boolean flag indicating if a reply has been sent.
	BaseReply func(res *Response) // BaseReply is a function to send a reply using the response object.
	ctx       context.Context     // ctx is the underlying context.
}

// Context returns the context associated with the BaseContext.
// If no context is set, it returns the background context.
func (c *BaseContext) Context() context.Context {
	if c.ctx == nil {
		return context.Background()
	}
	return c.ctx
}

// Reply sends a response to the client.
// It sets the response and calls the BaseReply method to handle the response.
// If the reply has already been sent, it returns false.
// Otherwise, it sets the reply status to true and returns true.
func (c *BaseContext) Reply(res *Response) bool {
	if !c.replyed.CompareAndSwap(false, true) {
		return false
	}

	c.setResponse(res)

	c.BaseReply(res)

	return true
}

// ReplyOK sends a successful response with the given data.
// It returns true if the response was sent successfully, otherwise false.
func (c *BaseContext) ReplyOK(data interface{}) bool {
	return c.Reply(&Response{
		Status: 200,
		Result: data,
	})
}

// ReplyError sends an error response with the specified status code and error message.
// It returns true if the response was successfully sent, otherwise false.
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

// GetResponse returns the response associated with the context.
// It acquires a lock on the response mutex to ensure thread safety.
// Returns the response object.
func (c *BaseContext) GetResponse() *Response {
	c.resMu.Lock()
	defer c.resMu.Unlock()
	return c.res
}

// Handler represents a reverse RPC handler.
// Method is the function to be executed when handling the request.
// Timeout is the maximum duration allowed for the request to complete.
type Handler struct {
	Method  func(c Context)
	Timeout time.Duration
}

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

// ChildContext represents the base interface for reverse RPC contexts.
type ChildContext interface {
	// ID returns the unique identifier of the context.
	ID() *ID

	// Method returns the name of the RPC method.
	Method() string

	// Reply sends a response message.
	// It returns true if the response was sent successfully, false otherwise.
	Reply(res *Response) bool

	// ReplyDesc returns the description of the reply message.
	ReplyDesc() string

	// Bind binds the request data to the context.
	Bind(request interface{}) error

	// PrometheusLabels returns the Prometheus labels associated with the context.
	PrometheusLabels() prometheus.Labels
}

// Context represents the context for reverse RPC.
type Context interface {
	ChildContext

	// Ctx returns the underlying context.Context.
	Ctx() context.Context

	// GetResponse returns the response message.
	GetResponse() *Response

	// ReplyOK sends a successful response message with the given data.
	// It returns true if the response was sent successfully, false otherwise.
	ReplyOK(data interface{}) bool

	// ReplyError sends an error response message with the given status and error.
	// It returns true if the response was sent successfully, false otherwise.
	ReplyError(status int, err error) bool
}

// RequestContext is the context implement for reverse RPC.
type RequestContext struct {
	res      *Response       // res is the response object.
	resMu    sync.Mutex      // resMu is a mutex to synchronize access to the response object.
	replyed  atomic.Bool     // replyed is an atomic boolean flag indicating if a reply has been sent.
	childCtx ChildContext    // childCtx is the underlying context instance.
	ctx      context.Context // ctx is the underlying context.
}

// NewRequestContext creates a new instance of RequestContext with the given context and ContextInstance.
// It returns a pointer to the newly created RequestContext.
func NewRequestContext(ctx context.Context, childCtx ChildContext) *RequestContext {
	return &RequestContext{
		ctx:      ctx,
		childCtx: childCtx,
	}
}

// ID returns the ID associated with the context.
func (c *RequestContext) ID() *ID {
	return c.childCtx.ID()
}

// Method returns the name of the RPC method being called.
// It retrieves the method name from the underlying context instance.
func (c *RequestContext) Method() string {
	return c.childCtx.Method()
}

// Bind binds the given request object to the context.
// It uses the underlying childCtx to perform the binding.
// Returns an error if the binding fails.
func (c *RequestContext) Bind(request interface{}) error {
	return c.childCtx.Bind(request)
}

// PrometheusLabels returns the Prometheus labels associated with the context.
// It retrieves the Prometheus labels from the underlying context instance.
func (c *RequestContext) PrometheusLabels() prometheus.Labels {
	return c.childCtx.PrometheusLabels()
}

// ReplyDesc returns the description of the reply message for the current context.
func (c *RequestContext) ReplyDesc() string {
	return c.childCtx.ReplyDesc()
}

// Ctx returns the context associated with the RequestContext.
// If no context is set, it returns the background context.
func (c *RequestContext) Ctx() context.Context {
	if c.ctx == nil {
		return context.Background()
	}
	return c.ctx
}

// Reply sends a response to the client.
// It sets the response and calls the BaseReply method to handle the response.
// If the reply has already been sent, it returns false.
// Otherwise, it sets the reply status to true and returns true.
func (c *RequestContext) Reply(res *Response) bool {
	if !c.replyed.CompareAndSwap(false, true) {
		return false
	}

	c.setResponse(res)

	return c.childCtx.Reply(res)
}

// ReplyOK sends a successful response with the given data.
// It returns true if the response was sent successfully, otherwise false.
func (c *RequestContext) ReplyOK(data interface{}) bool {
	return c.Reply(&Response{
		Status: 200,
		Result: data,
	})
}

// ReplyError sends an error response with the specified status code and error message.
// It returns true if the response was successfully sent, otherwise false.
func (c *RequestContext) ReplyError(status int, err error) bool {
	return c.Reply(&Response{
		Status: status,
		Error:  err,
	})
}

func (c *RequestContext) setResponse(res *Response) {
	c.resMu.Lock()
	defer c.resMu.Unlock()
	c.res = res
}

// GetResponse returns the response associated with the context.
// It acquires a lock on the response mutex to ensure thread safety.
// Returns the response object.
func (c *RequestContext) GetResponse() *Response {
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

package mqttjson

import (
	"context"
	"encoding/json"

	"github.com/go-playground/validator/v10"
	rrpc "github.com/xizhibei/go-reverse-rpc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// MQTTContext represents the context of an MQTT request.
type MQTTContext struct {
	req       *RequestData
	service   *Server
	validator *validator.Validate
	ctx       context.Context
}

// NewMQTTContext creates a new MQTTContext with the given request, service, and validator.
// It returns a pointer to the created MQTTContext.
func NewMQTTContext(req *RequestData, service *Server, validator *validator.Validate) *MQTTContext {
	ctx := context.Background()
	if req.Request.Metadata != nil {
		// Extract trace context using OpenTelemetry propagator
		propagator := otel.GetTextMapPropagator()
		carrier := propagation.MapCarrier(req.Request.Metadata)
		ctx = propagator.Extract(ctx, carrier)
	}
	return &MQTTContext{
		req:       req,
		service:   service,
		validator: validator,
		ctx:       ctx,
	}
}

// ID returns the ID of the MQTTContext.
func (c *MQTTContext) ID() *rrpc.ID {
	return &rrpc.ID{Num: c.req.ID}
}

// ReplyDesc returns the reply topic for the MQTTContext.
func (c *MQTTContext) ReplyDesc() string {
	return c.req.ReplyTopic()
}

// Method returns the method of the MQTTContext.
// It retrieves the method from the underlying request.
func (c *MQTTContext) Method() string {
	return c.req.Method
}

// Ctx returns the context of the MQTTContext.
func (c *MQTTContext) Ctx() context.Context {
	return c.ctx
}

// Bind unmarshals the JSON-encoded request parameters into the provided request object
// and validates the request using the validator. It returns an error if the unmarshaling
// or validation fails.
func (c *MQTTContext) Bind(request interface{}) error {
	err := json.Unmarshal(c.req.Params, request)
	if err != nil {
		return err
	}

	err = c.validator.Struct(request)
	if err != nil {
		return err
	}
	return nil
}

// Reply sends a response to the client.
// If the response has an error, it sends an error response.
// Otherwise, it sends an OK response with the result.
// It returns true to indicate that the response was sent successfully.
func (c *MQTTContext) Reply(res *rrpc.Response) bool {
	if res.Error != nil {
		_ = c.service.reply(c.req.MakeErrResponse(res.Status, res.Error))
		return true
	}
	_ = c.service.reply(c.req.MakeOKResponse(res.Result))
	return true
}

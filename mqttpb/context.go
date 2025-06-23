package mqttpb

import (
	"context"
	"reflect"

	"github.com/cockroachdb/errors"
	rrpc "github.com/xizhibei/go-reverse-rpc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

var (
	// ErrRequestNotProto is an error indicating that the request type is not a protobuf message.
	ErrRequestNotProto = errors.New("[RRPC] request type is not protobuf message")
)

// MQTTContext represents the context of an MQTT request.
type MQTTContext struct {
	req *RequestData
	svc *Server
	ctx context.Context
}

// NewMQTTContext creates a new MQTTContext instance.
func NewMQTTContext(req *RequestData, svc *Server) *MQTTContext {
	ctx := context.Background()
	if req.Request.Metadata != nil {
		// Extract trace context using OpenTelemetry propagator
		propagator := otel.GetTextMapPropagator()
		carrier := propagation.MapCarrier(req.Request.Metadata)
		ctx = propagator.Extract(ctx, carrier)
	}

	return &MQTTContext{
		req: req,
		svc: svc,
		ctx: ctx,
	}
}

// ID returns the ID of the MQTTContext.
func (c *MQTTContext) ID() *rrpc.ID {
	return &rrpc.ID{Num: c.req.Id}
}

// Method returns the method of the MQTTContext.
func (c *MQTTContext) Method() string {
	return c.req.Method
}

// Metadata returns the metadata of the MQTTContext.
func (c *MQTTContext) Metadata() map[string]string {
	return c.req.Metadata
}

// Ctx returns the context of the MQTTContext.
func (c *MQTTContext) Ctx() context.Context {
	return c.ctx
}

// MetadataGetter is an interface for getting metadata.
type MetadataGetter interface {
	Metadata() map[string]string
}

// Encoding returns the content encoding of the MQTTContext.
func (c *MQTTContext) Encoding() ContentEncoding {
	return c.req.Encoding
}

// EncodingGetter is an interface for getting the content encoding.
type EncodingGetter interface {
	Encoding() ContentEncoding
}

// ReplyDesc returns the reply description of the MQTTContext.
func (c *MQTTContext) ReplyDesc() string {
	return c.req.GetReplyTopic()
}

// Bind binds the request data to the given request object.
func (c *MQTTContext) Bind(request interface{}) error {
	m, ok := request.(proto.Message)
	if !ok {
		return ErrRequestNotProto
	}
	typeURL := reflect.TypeOf(m).String()
	if typeURL != c.req.Body.TypeUrl {
		return errors.Newf("[RRPC] type %s is not found", typeURL)
	}

	return proto.Unmarshal(c.req.Body.Value, m)
}

// Reply sends the response back to the client.
func (c *MQTTContext) Reply(res *rrpc.Response) bool {
	if res.Error != nil {
		_ = c.svc.reply(c.req.MakeErrResponse(res.Status, res.Error))
		return true
	}
	if result, ok := res.Result.(proto.Message); ok {
		_ = c.svc.reply(c.req.MakeOKResponse(result))
	} else {
		_ = c.svc.reply(c.req.MakeOKResponse(&emptypb.Empty{}))
	}
	return true
}

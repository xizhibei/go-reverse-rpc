package mqtt_pb_server

import (
	"reflect"

	"github.com/cockroachdb/errors"
	"github.com/prometheus/client_golang/prometheus"
	rrpc "github.com/xizhibei/go-reverse-rpc"
	"github.com/xizhibei/go-reverse-rpc/pb_encoding"
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
	svc *MqttServer
}

// NewMQTTContext creates a new MQTTContext instance.
func NewMQTTContext(req *RequestData, svc *MqttServer) *MQTTContext {
	ctx := MQTTContext{
		req: req,
		svc: svc,
	}

	return &ctx
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

// MetadataGetter is an interface for getting metadata.
type MetadataGetter interface {
	Metadata() map[string]string
}

// Encoding returns the content encoding of the MQTTContext.
func (c *MQTTContext) Encoding() pb_encoding.ContentEncoding {
	return c.req.Encoding
}

// EncodingGetter is an interface for getting the content encoding.
type EncodingGetter interface {
	Encoding() pb_encoding.ContentEncoding
}

// PrometheusLabels returns the Prometheus labels of the MQTTContext.
func (c *MQTTContext) PrometheusLabels() prometheus.Labels {
	r := c.svc.iotClient.GetClientOptions()
	uri := r.Servers[0]
	return prometheus.Labels{
		"method": c.req.Method,
		"host":   uri.Host,
	}
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

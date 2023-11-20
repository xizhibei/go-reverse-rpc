package mqtt_pb_server

import (
	"reflect"

	"github.com/cockroachdb/errors"
	"github.com/prometheus/client_golang/prometheus"
	reverse_rpc "github.com/xizhibei/go-reverse-rpc"
	"github.com/xizhibei/go-reverse-rpc/reverse_rpc_pb/pb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

var (
	ErrRequestNotProto = errors.New("Request type is not proto message")
)

type MQTTContext struct {
	reverse_rpc.BaseContext
	req *RequestData
	svc *Service
}

func NewMQTTContext(req *RequestData, svc *Service) *MQTTContext {
	ctx := MQTTContext{
		req: req,
		svc: svc,
	}
	ctx.BaseContext.BaseReply = ctx.reply
	return &ctx
}

func (c *MQTTContext) ID() *reverse_rpc.ID {
	return &reverse_rpc.ID{Num: c.req.Id}
}

func (c *MQTTContext) Method() string {
	return c.req.Method
}

func (c *MQTTContext) Metadata() map[string]string {
	return c.req.Metadata
}

type MetadataGetter interface {
	Metadata() map[string]string
}

func (c *MQTTContext) Encoding() pb.ContentEncoding {
	return c.req.Encoding
}

type EncodingGetter interface {
	Encoding() pb.ContentEncoding
}

func (c *MQTTContext) PrometheusLabels() prometheus.Labels {
	return prometheus.Labels{
		"method": c.req.Method,
		"host":   c.svc.host,
	}
}

func (c *MQTTContext) ReplyDesc() string {
	return c.req.GetReplyTopic()
}

func (c *MQTTContext) Bind(request interface{}) error {
	m, ok := request.(proto.Message)
	if !ok {
		return ErrRequestNotProto
	}
	typeURL := reflect.TypeOf(m).String()
	if typeURL != c.req.Body.TypeUrl {
		return errors.Newf("Type %s is not found", typeURL)
	}

	return proto.Unmarshal(c.req.Body.Value, m)
}

func (c *MQTTContext) reply(res *reverse_rpc.Response) {
	if res.Error != nil {
		_ = c.svc.reply(c.req.MakeErrResponse(res.Status, res.Error))
		return
	}
	if result, ok := res.Result.(proto.Message); ok {
		_ = c.svc.reply(c.req.MakeOKResponse(result))
	} else {
		_ = c.svc.reply(c.req.MakeOKResponse(&emptypb.Empty{}))
	}
}

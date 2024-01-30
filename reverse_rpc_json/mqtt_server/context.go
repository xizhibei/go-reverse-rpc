package mqtt_json_server

import (
	"encoding/json"

	"github.com/go-playground/validator/v10"
	"github.com/prometheus/client_golang/prometheus"
	reverse_rpc "github.com/xizhibei/go-reverse-rpc"
)

type MQTTContext struct {
	req       *Request
	service   *Service
	validator *validator.Validate
}

func NewMQTTContext(req *Request, service *Service, validator *validator.Validate) *MQTTContext {
	ctx := MQTTContext{
		req:       req,
		service:   service,
		validator: validator,
	}
	return &ctx
}

func (c *MQTTContext) ID() *reverse_rpc.ID {
	return &reverse_rpc.ID{Num: c.req.ID}
}

func (c *MQTTContext) ReplyDesc() string {
	return c.req.ReplyTopic()
}

func (c *MQTTContext) Method() string {
	return c.req.Method
}

func (c *MQTTContext) PrometheusLabels() prometheus.Labels {
	return prometheus.Labels{
		"method": c.req.Method,
		"host":   c.service.host,
	}
}

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

func (c *MQTTContext) Reply(res *reverse_rpc.Response) bool {
	if res.Error != nil {
		_ = c.service.reply(c.req.MakeErrResponse(res.Status, res.Error))
		return true
	}
	_ = c.service.reply(c.req.MakeOKResponse(res.Result))
	return true
}

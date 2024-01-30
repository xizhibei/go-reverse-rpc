package websocket_json_server

import (
	"encoding/json"

	"github.com/gin-gonic/gin/binding"
	"github.com/prometheus/client_golang/prometheus"
	reverse_rpc "github.com/xizhibei/go-reverse-rpc"
)

type WSContext struct {
	req     *Request
	service *Service
}

func NewWSContext(req *Request, service *Service) *WSContext {
	ctx := WSContext{
		req:     req,
		service: service,
	}
	return &ctx
}

func (c *WSContext) ID() *reverse_rpc.ID {
	return &reverse_rpc.ID{Num: c.req.ID}
}

func (c *WSContext) Method() string {
	return c.req.Method
}

func (c *WSContext) PrometheusLabels() prometheus.Labels {
	return prometheus.Labels{
		"method": c.req.Method,
		"host":   c.service.host,
	}
}

func (c *WSContext) ReplyDesc() string {
	return c.Method() + c.ID().String()
}

func (c *WSContext) Bind(request interface{}) error {
	err := json.Unmarshal(c.req.Params, request)
	if err != nil {
		return err
	}

	err = binding.Validator.ValidateStruct(request)
	if err != nil {
		return err
	}
	return nil
}

func (c *WSContext) Reply(res *reverse_rpc.Response) bool {
	if res.Error != nil {
		_ = c.service.reply(c.req.MakeErrResponse(res.Status, res.Error))
		return true
	}
	_ = c.service.reply(c.req.MakeOKResponse(res.Result))
	return true
}

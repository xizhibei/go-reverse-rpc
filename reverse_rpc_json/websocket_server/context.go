package websocket_json_server

import (
	"encoding/json"

	"github.com/gin-gonic/gin/binding"
	"github.com/prometheus/client_golang/prometheus"
	reverse_rpc "github.com/xizhibei/go-reverse-rpc"
)

// WSContext represents the context of a websocket request.
type WSContext struct {
	req     *Request
	service *Service
}

// NewWSContext creates a new WSContext object.
// It takes a pointer to a Request object and a pointer to a Service object as parameters.
// It returns a pointer to the newly created WSContext object.
func NewWSContext(req *Request, service *Service) *WSContext {
	ctx := WSContext{
		req:     req,
		service: service,
	}
	return &ctx
}

// ID returns the ID of the WSContext.
func (c *WSContext) ID() *reverse_rpc.ID {
	return &reverse_rpc.ID{Num: c.req.ID}
}

// Method returns the HTTP method of the WebSocket request.
func (c *WSContext) Method() string {
	return c.req.Method
}

// PrometheusLabels returns the Prometheus labels for the WebSocket context.
// It includes the method and host labels.
func (c *WSContext) PrometheusLabels() prometheus.Labels {
	return prometheus.Labels{
		"method": c.req.Method,
		"host":   c.service.host,
	}
}

// ReplyDesc returns a string that represents the description of the reply.
// It concatenates the method name and the ID of the context.
func (c *WSContext) ReplyDesc() string {
	return c.Method() + c.ID().String()
}

// Bind decodes the JSON-RPC request parameters into the provided request object.
// It uses the json.Unmarshal function to perform the decoding.
// After decoding, it validates the request object using the binding.Validator.ValidateStruct function.
// If any decoding or validation error occurs, it returns the error; otherwise, it returns nil.
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

// Reply sends a response to the client over the WebSocket connection.
// If the response contains an error, it sends an error response.
// Otherwise, it sends an OK response with the result.
// It returns true to indicate that the response was sent successfully.
func (c *WSContext) Reply(res *reverse_rpc.Response) bool {
	if res.Error != nil {
		_ = c.service.reply(c.req.MakeErrResponse(res.Status, res.Error))
		return true
	}
	_ = c.service.reply(c.req.MakeOKResponse(res.Result))
	return true
}

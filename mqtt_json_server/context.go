package mqtt_json_server

import (
	"encoding/json"

	"github.com/go-playground/validator/v10"
	"github.com/prometheus/client_golang/prometheus"
	reverse_rpc "github.com/xizhibei/go-reverse-rpc"
)

// MQTTContext represents the context of an MQTT request.
type MQTTContext struct {
	req       *Request
	service   *MqttServer
	validator *validator.Validate
}

// NewMQTTContext creates a new MQTTContext with the given request, service, and validator.
// It returns a pointer to the created MQTTContext.
func NewMQTTContext(req *Request, service *MqttServer, validator *validator.Validate) *MQTTContext {
	ctx := MQTTContext{
		req:       req,
		service:   service,
		validator: validator,
	}
	return &ctx
}

// ID returns the ID of the MQTTContext.
func (c *MQTTContext) ID() *reverse_rpc.ID {
	return &reverse_rpc.ID{Num: c.req.ID}
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

// PrometheusLabels returns the Prometheus labels for the MQTTContext.
// It includes the method and host labels.
func (c *MQTTContext) PrometheusLabels() prometheus.Labels {
	r := c.service.iotClient.GetClientOptions()
	uri := r.Servers[0]
	return prometheus.Labels{
		"method": c.req.Method,
		"host":   uri.Host,
	}
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
func (c *MQTTContext) Reply(res *reverse_rpc.Response) bool {
	if res.Error != nil {
		_ = c.service.reply(c.req.MakeErrResponse(res.Status, res.Error))
		return true
	}
	_ = c.service.reply(c.req.MakeOKResponse(res.Result))
	return true
}

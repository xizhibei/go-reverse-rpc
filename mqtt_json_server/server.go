package mqtt_json_server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
	reverse_rpc "github.com/xizhibei/go-reverse-rpc"
	"github.com/xizhibei/go-reverse-rpc/json_encoding"
	"github.com/xizhibei/go-reverse-rpc/mqtt_adapter"

	"go.uber.org/zap"
)

var (
	// ErrRetailedMessage is an error indicating that retain message is not allowed.
	ErrRetailedMessage = errors.New("[RRPC] reatain message is not allowed, please set retaind=false")
)

// MqttServer represents a MQTT service.
type MqttServer struct {
	*reverse_rpc.Server
	iotClient mqtt_adapter.MQTTClientAdapter
	log       *zap.SugaredLogger
	validator *validator.Validate

	subscribeTopic string
	qos            byte
}

// New creates a new MQTT server with the provided options.
// It initializes an MQTT client with the given MQTT options and connects to the MQTT broker.
// The function also sets up the necessary configurations for the reverse RPC server.
// It returns a pointer to the created Service and an error if any.
func New(client mqtt_adapter.MQTTClientAdapter, subscribeTopic string, validator *validator.Validate, options ...reverse_rpc.ServerOption) *MqttServer {
	s := MqttServer{
		Server:         reverse_rpc.NewServer(options...),
		iotClient:      client,
		subscribeTopic: subscribeTopic,
		log:            zap.S().With("module", "rrpc.mqtt_json_server"),
		validator:      validator,
	}

	client.EnsureConnected()

	client.OnConnect(func() {
		err := s.initReceive()
		if err != nil {
			s.log.Errorf("init receive %v", err)
		}
	})
	return &s
}

// Close closes the MQTT service by disconnecting the IoT client.
// It returns an error if there was a problem disconnecting the client.
func (s *MqttServer) Close() error {
	s.iotClient.Disconnect()
	return nil
}

// Request represents a request message received by the service.
type Request struct {
	Topic string
	json_encoding.Request
}

// ReplyTopic returns the topic for the response message corresponding to the request.
// It replaces the word "request" with "response" in the original topic.
func (r *Request) ReplyTopic() string {
	return strings.ReplaceAll(r.Topic, "request", "response")
}

// GetResponse returns the response object for the request.
// It creates a new Response object with the reply topic and sets the ID and Method fields.
func (r *Request) GetResponse() *Response {
	replyTopic := r.ReplyTopic()
	return &Response{
		Topic: replyTopic,
		Response: json_encoding.Response{
			ID:     r.ID,
			Method: r.Method,
		},
	}
}

// MakeOKResponse creates a successful response with status code 200 and the provided data.
// It marshals the data to JSON format and sets it as the response data.
// The response object is returned.
func (r *Request) MakeOKResponse(x interface{}) *Response {
	res := r.GetResponse()
	res.Status = reverse_rpc.RPCStatusOK
	data, _ := json.Marshal(x)

	res.Data = data
	return res
}

// MakeErrResponse creates an error response with the specified status code and error message.
// It returns a pointer to a Response object.
func (r *Request) MakeErrResponse(status int, err error) *Response {
	res := r.GetResponse()
	res.Status = status
	data, _ := json.Marshal(map[string]string{
		"message": err.Error(),
	})
	res.Data = data
	fmt.Printf("%+v\n", err)
	return res
}

// Response represents a response message sent by the service.
type Response struct {
	Topic string
	json_encoding.Response
}

func (s *MqttServer) reply(res *Response) error {
	data, err := json.Marshal(res.Response)
	if err != nil {
		return err
	}
	s.log.Infof("Response to topic %s, method %s size %d", res.Topic, res.Method, len(data))
	s.iotClient.PublishBytes(context.TODO(), res.Topic, s.qos, false, data)

	return nil
}

func (s *MqttServer) initReceive() error {
	s.iotClient.Subscribe(context.TODO(), s.subscribeTopic, s.qos, func(client mqtt_adapter.MQTTClientAdapter, m mqtt_adapter.Message) {
		req := Request{
			Topic: m.Topic(),
		}

		if m.Retained() {
			s.log.Errorf("Retained message, ignore")
			_ = s.reply(req.MakeErrResponse(400, ErrRetailedMessage))
			return
		}

		err := json.Unmarshal(m.Payload(), &req.Request)
		if err != nil {
			s.log.Errorf("Parse json %v", err)
			_ = s.reply(req.MakeErrResponse(400, err))
			return
		}

		s.log.Debugf("Request from topic %s, method %s", m.Topic(), req.Method)

		mqttCtx := NewMQTTContext(&req, s, s.validator)
		c := reverse_rpc.NewRequestContext(context.Background(), mqttCtx)

		s.Server.Call(c)
	})
	return nil
}

// IsConnected returns a boolean value indicating whether the service is connected to the MQTT broker.
func (s *MqttServer) IsConnected() bool {
	return s.iotClient.IsConnected()
}
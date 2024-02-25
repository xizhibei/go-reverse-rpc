package mqttjson

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
	rrpc "github.com/xizhibei/go-reverse-rpc"
	"github.com/xizhibei/go-reverse-rpc/mqttadapter"

	"go.uber.org/zap"
)

var (
	// ErrRetailedMessage is an error indicating that retain message is not allowed.
	ErrRetailedMessage = errors.New("[RRPC] reatain message is not allowed, please set retaind=false")
)

// Server represents a MQTT service.
type Server struct {
	*rrpc.Server
	iotClient mqttadapter.MQTTClientAdapter
	log       *zap.SugaredLogger
	validator *validator.Validate

	subscribeTopic string
	qos            byte
}

// NewServer creates a new MQTT server with the provided options.
// It initializes an MQTT client with the given MQTT options and connects to the MQTT broker.
// The function also sets up the necessary configurations for the reverse RPC server.
// It returns a pointer to the created Service and an error if any.
func NewServer(client mqttadapter.MQTTClientAdapter, subscribeTopic string, validator *validator.Validate, options ...rrpc.ServerOption) *Server {
	s := Server{
		Server:         rrpc.NewServer(options...),
		iotClient:      client,
		subscribeTopic: subscribeTopic,
		log:            zap.S().With("module", "rrpc.mqttjsonserver"),
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
func (s *Server) Close() error {
	s.iotClient.Disconnect()
	return nil
}

type request struct {
	Topic string
	Request
}

// ReplyTopic returns the topic for the response message corresponding to the request.
// It replaces the word "request" with "response" in the original topic.
func (r *request) ReplyTopic() string {
	return strings.ReplaceAll(r.Topic, "request", "response")
}

// GetResponse returns the response object for the request.
// It creates a new Response object with the reply topic and sets the ID and Method fields.
func (r *request) GetResponse() *response {
	replyTopic := r.ReplyTopic()
	return &response{
		Topic: replyTopic,
		Response: Response{
			ID:     r.ID,
			Method: r.Method,
		},
	}
}

// MakeOKResponse creates a successful response with status code 200 and the provided data.
// It marshals the data to JSON format and sets it as the response data.
// The response object is returned.
func (r *request) MakeOKResponse(x interface{}) *response {
	res := r.GetResponse()
	res.Status = rrpc.RPCStatusOK
	data, _ := json.Marshal(x)

	res.Data = data
	return res
}

// MakeErrResponse creates an error response with the specified status code and error message.
// It returns a pointer to a Response object.
func (r *request) MakeErrResponse(status int, err error) *response {
	res := r.GetResponse()
	res.Status = status
	data, _ := json.Marshal(map[string]string{
		"message": err.Error(),
	})
	res.Data = data
	fmt.Printf("%+v\n", err)
	return res
}

type response struct {
	Topic string
	Response
}

func (s *Server) reply(res *response) error {
	data, err := json.Marshal(res.Response)
	if err != nil {
		return err
	}
	s.log.Infof("Response to topic %s, method %s size %d", res.Topic, res.Method, len(data))
	s.iotClient.PublishBytes(context.TODO(), res.Topic, s.qos, false, data)

	return nil
}

func (s *Server) initReceive() error {
	s.iotClient.Subscribe(context.TODO(), s.subscribeTopic, s.qos, func(client mqttadapter.MQTTClientAdapter, m mqttadapter.Message) {
		req := request{
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
		c := rrpc.NewRequestContext(context.Background(), mqttCtx)

		s.Server.Call(c)
	})
	return nil
}

// IsConnected returns a boolean value indicating whether the service is connected to the MQTT broker.
func (s *Server) IsConnected() bool {
	return s.iotClient.IsConnected()
}

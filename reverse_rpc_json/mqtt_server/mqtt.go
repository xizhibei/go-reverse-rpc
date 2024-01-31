package mqtt_json_server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	reverse_rpc "github.com/xizhibei/go-reverse-rpc"
	"github.com/xizhibei/go-reverse-rpc/mqtt"
	"github.com/xizhibei/go-reverse-rpc/reverse_rpc_json"

	"go.uber.org/zap"
)

var (
	// ErrRetailedMessage is an error indicating that retain message is not allowed.
	ErrRetailedMessage = errors.New("[RRPC] reatain message is not allowed, please set retaind=false")
)

// MQTTOptions is the options for MQTT.
type MQTTOptions struct {
	Uri            string
	User           string
	Pass           string
	ClientID       string
	Topic          string
	Qos            byte
	KeepAlive      time.Duration
	EnableStatus   bool
	StatusTopic    string
	OnlinePayload  []byte
	OfflinePayload []byte
}

// Service represents a MQTT service.
type Service struct {
	*reverse_rpc.Server
	iotClient *mqtt.Client
	log       *zap.SugaredLogger
	validator *validator.Validate

	host  string
	topic string
	qos   byte
}

// New creates a new MQTT service with the provided options.
// It initializes an MQTT client with the given MQTT options and connects to the MQTT broker.
// The function also sets up the necessary configurations for the reverse RPC server.
// It returns a pointer to the created Service and an error if any.
func New(opts *MQTTOptions, validator *validator.Validate, options ...reverse_rpc.ServerOption) (*Service, error) {
	iotOptions := []mqtt.Option{
		mqtt.WithUserPass(opts.User, opts.Pass),
	}
	if opts.KeepAlive > 0 {
		iotOptions = append(iotOptions, mqtt.WithKeepAlive(opts.KeepAlive))
	}
	if opts.EnableStatus {
		iotOptions = append(iotOptions, mqtt.WithStatus(
			opts.StatusTopic, opts.OnlinePayload,
			opts.StatusTopic, opts.OfflinePayload,
		))
	}
	client, err := mqtt.NewClient(opts.Uri, opts.ClientID, iotOptions...)
	if err != nil {
		return nil, err
	}

	parsedURI, err := url.Parse(opts.Uri)
	if err != nil {
		return nil, err
	}
	parsedURI.User = nil

	s := Service{
		Server:    reverse_rpc.NewServer(options...),
		iotClient: client,
		topic:     opts.Topic,
		qos:       opts.Qos,
		host:      parsedURI.String(),
		log:       zap.S().With("module", "reverse_rpc.mqtt"),
		validator: validator,
	}

	client.EnsureConnected()

	client.OnConnect(func() {
		err := s.initReceive()
		if err != nil {
			s.log.Errorf("init receive %v", err)
		}
	})
	return &s, nil
}

// Close closes the MQTT service by disconnecting the IoT client.
// It returns an error if there was a problem disconnecting the client.
func (s *Service) Close() error {
	s.iotClient.Disconnect()
	return nil
}

// Request represents a request message received by the service.
type Request struct {
	Topic string
	reverse_rpc_json.Request
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
		Response: reverse_rpc_json.Response{
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
	res.Status = 200
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
	reverse_rpc_json.Response
}

func (s *Service) reply(res *Response) error {
	data, err := json.Marshal(res.Response)
	if err != nil {
		return err
	}
	s.log.Infof("Response to topic %s, method %s size %d", res.Topic, res.Method, len(data))
	tk := s.iotClient.PublishBytes(res.Topic, s.qos, false, data)
	if tk.Error() != nil {
		return tk.Error()
	}

	return nil
}

func (s *Service) initReceive() error {
	token := s.iotClient.Subscribe(s.topic, s.qos, func(client *mqtt.Client, m mqtt.Message) {
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

		// s.log.Infof("Request from topic %s, method %s", m.Topic, req.Method)

		mqttCtx := NewMQTTContext(&req, s, s.validator)
		c := reverse_rpc.NewRequestContext(context.Background(), mqttCtx)

		s.Server.Call(c)
	})
	return token.Error()
}

// IsConnected returns a boolean value indicating whether the service is connected to the MQTT broker.
func (s *Service) IsConnected() bool {
	return s.iotClient.IsConnected()
}

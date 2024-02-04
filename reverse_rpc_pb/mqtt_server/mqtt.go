package mqtt_pb_server

import (
	"context"
	"reflect"
	"strings"

	reverse_rpc "github.com/xizhibei/go-reverse-rpc"
	"github.com/xizhibei/go-reverse-rpc/mqtt"
	"github.com/xizhibei/go-reverse-rpc/reverse_rpc_pb"
	rrpcpb "github.com/xizhibei/go-reverse-rpc/reverse_rpc_pb/pb"

	"github.com/cockroachdb/errors"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

var (
	// ErrUnknownContentEncoding is an error indicating an unknown content encoding.
	ErrUnknownContentEncoding = errors.New("[RRPC] unknown content encoding")
)

// Service represents a MQTT service.
type Service struct {
	*reverse_rpc.Server
	iotClient *mqtt.Client
	codec     *reverse_rpc_pb.ServerCodec
	log       *zap.SugaredLogger

	subscribeTopic string
}

// New creates a new Service instance with the provided MQTT client and options.
// It returns a pointer to the Service and an error, if any.
func New(client *mqtt.Client, subscribeTopic string, options ...reverse_rpc.ServerOption) *Service {
	s := Service{
		Server:         reverse_rpc.NewServer(options...),
		iotClient:      client,
		subscribeTopic: subscribeTopic,
		codec:          reverse_rpc_pb.NewServerCodec(),
		log:            zap.S().With("module", "rrpc.pb.mqtt.server"),
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

// IsConnected returns a boolean value indicating whether the service is connected to the IoT client.
func (s *Service) IsConnected() bool {
	return s.iotClient.IsConnected()
}

// Close closes the service by disconnecting the IoT client.
// It returns an error if there was a problem disconnecting the client.
func (s *Service) Close() error {
	s.iotClient.Disconnect()
	return nil
}

// RequestData represents a request received by the service.
type RequestData struct {
	Topic string
	rrpcpb.Request
}

// ResponseData represents a response sent by the service.
type ResponseData struct {
	Topic string
	rrpcpb.Response
}

// GetResponse returns the response data for the request.
// It creates a new ResponseData object with the reply topic and the request's ID and encoding.
func (r *RequestData) GetResponse() *ResponseData {
	return &ResponseData{
		Topic: r.GetReplyTopic(),
		Response: rrpcpb.Response{
			Id:       r.Id,
			Encoding: r.Encoding,
		},
	}
}

// MakeOKResponse creates a successful response with the given data.
// It marshals the data into bytes using protocol buffers and sets the response status to 200.
// The data is stored in the response body as an Any message, with the type URL set to the type of the data.
// If an error occurs during marshaling, it returns nil.
func (r *RequestData) MakeOKResponse(data proto.Message) *ResponseData {
	res := r.GetResponse()
	res.Status = reverse_rpc.RPCStatusOK
	d, err := proto.Marshal(data)
	if err != nil {
		return nil
	}
	res.Body = &anypb.Any{
		TypeUrl: reflect.TypeOf(data).String(),
		Value:   d,
	}

	return res
}

// MakeErrResponse creates an error response with the specified status code and error message.
// It sets the status code and error message in the response data and returns the modified response.
func (r *RequestData) MakeErrResponse(status int, err error) *ResponseData {
	res := r.GetResponse()
	res.Status = int32(status)
	res.ErrorMessage = err.Error()
	return res
}

// GetReplyTopic returns the reply topic for the request.
// It replaces the word "request" with "response" in the original topic.
func (r *RequestData) GetReplyTopic() string {
	return strings.Replace(r.Topic, "request", "response", 1)
}

func (s *Service) reply(res *ResponseData) error {
	if res.Status != reverse_rpc.RPCStatusOK {
		s.log.Errorf("ResponseData error %#v", res)
	}
	data, err := s.codec.Marshal(&res.Response)
	if err != nil {
		return err
	}
	_ = s.iotClient.PublishBytes(res.Topic, reverse_rpc.DefaultQoS, false, data)
	return nil
}

func (s *Service) initReceive() error {
	token := s.iotClient.Subscribe(s.subscribeTopic, reverse_rpc.DefaultQoS, func(client *mqtt.Client, m mqtt.Message) {
		s.log.Debugf("Request from json pb topic %s, method %s", m.Topic(), "Subscribe")
		req := RequestData{
			Topic: m.Topic(),
		}

		err := s.codec.Unmarshal(m.Payload(), &req.Request)
		if err != nil {
			s.log.Errorf("Parse body %+v", err)
			_ = s.reply(req.MakeErrResponse(400, err))
			return
		}

		s.log.Debugf("Request from pb  topic %s, method %s", m.Topic(), req.Method)

		mqttCtx := NewMQTTContext(&req, s)
		c := reverse_rpc.NewRequestContext(context.Background(), mqttCtx)

		go s.Server.Call(c)
	})
	return token.Error()
}

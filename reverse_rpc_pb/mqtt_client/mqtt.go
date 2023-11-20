package mqtt_pb_client

import (
	"context"
	"net/rpc"
	"path"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/xizhibei/go-reverse-rpc/mqtt"
	"github.com/xizhibei/go-reverse-rpc/reverse_rpc_pb"
	"github.com/xizhibei/go-reverse-rpc/reverse_rpc_pb/pb"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

var (
	ErrClientIsNotReady = errors.New("Client is not ready")
)

type Client struct {
	mqttClient         *mqtt.Client
	log                *zap.SugaredLogger
	rpcClientCodecPool sync.Pool

	topicPrefix string
	qos         byte
}

func New(uri, clientID, topicPrefix string, options ...mqtt.Option) (*Client, error) {
	client, err := mqtt.NewClient(uri, clientID, options...)
	if err != nil {
		return nil, err
	}

	s := NewWithMQTTClient(topicPrefix, 0, pb.ContentEncoding_BROTLI, client)

	client.ConnectForever()

	return s, nil
}

func NewWithMQTTClient(topicPrefix string, qos byte, encoding pb.ContentEncoding, client *mqtt.Client) *Client {
	s := Client{
		mqttClient:  client,
		topicPrefix: topicPrefix,
		log:         zap.S().With("module", "reverse_rpc.mqtt"),
		rpcClientCodecPool: sync.Pool{
			New: func() interface{} {
				return reverse_rpc_pb.NewRPCClientCodec(encoding)
			},
		},
	}

	return &s
}

func (s *Client) Client() *mqtt.Client {
	return s.mqttClient
}

func (s *Client) OnConnect(cb func()) int {
	return s.mqttClient.OnConnect(cb)
}

func (s *Client) IsConnected() bool {
	return s.mqttClient.IsConnected()
}

func (s *Client) Close() error {
	s.mqttClient.Disconnect()
	return nil
}

func (s *Client) Subscribe(topic string, qos byte, cb mqtt.MessageCallback) {
	s.mqttClient.Subscribe(topic, qos, cb)
}

func (s *Client) createRPCClient(deviceID string, encoding pb.ContentEncoding) (*rpc.Client, func(), error) {
	id := uuid.NewString()
	requestTopic := path.Join(s.topicPrefix, deviceID, "request", id)
	responseTopic := path.Join(s.topicPrefix, deviceID, "response", id)
	conn, err := NewConn(requestTopic, responseTopic, s.mqttClient, s.qos)
	if err != nil {
		return nil, nil, err
	}

	codec := s.rpcClientCodecPool.Get().(*reverse_rpc_pb.RPCClientCodec)
	codec.Reset(conn)
	codec.SetEncoding(encoding)

	rpcClient := rpc.NewClientWithCodec(codec)

	return rpcClient, func() {
		rpcClient.Close()
		s.rpcClientCodecPool.Put(codec)
	}, nil
}

type callOpt struct {
	encoding pb.ContentEncoding
}

type CallOption func(o *callOpt)

func WithEncoding(encoding pb.ContentEncoding) CallOption {
	return func(o *callOpt) {
		o.encoding = encoding
	}
}

func (s *Client) Call(ctx context.Context, deviceID, serviceMethod string, args proto.Message, reply proto.Message, opts ...CallOption) error {
	var callOpt callOpt
	for _, o := range opts {
		o(&callOpt)
	}

	rpcClient, close, err := s.createRPCClient(deviceID, callOpt.encoding)
	if err != nil {
		return err
	}
	defer close()

	call := rpcClient.Go(serviceMethod, args, reply, nil)

	select {
	case <-call.Done:
		return call.Error
	case <-ctx.Done():
		return ctx.Err()
	}
}

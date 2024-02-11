package mqtt_pb_client

import (
	"context"
	"net/rpc"
	"path"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	reverse_rpc "github.com/xizhibei/go-reverse-rpc"
	"github.com/xizhibei/go-reverse-rpc/mqtt_adapter"
	"github.com/xizhibei/go-reverse-rpc/pb_encoding"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

var (
	// ErrClientIsNotReady is an error indicating that the client is not ready.
	ErrClientIsNotReady = errors.New("[RRPC] client is not ready")
)

// Client represents an MQTT client used for reverse RPC communication.
type Client struct {
	mqttClient         mqtt_adapter.MQTTClientAdapter
	log                *zap.SugaredLogger
	rpcClientCodecPool sync.Pool

	topicPrefix string
}

// New creates a new instance of the Client struct with the provided parameters.
// It initializes the MQTT client, sets the topic prefix, quality of service (QoS), content encoding,
// and initializes the RPC client codec pool.
// The returned pointer to the Client struct can be used to interact with the MQTT client.
func New(client mqtt_adapter.MQTTClientAdapter, topicPrefix string, encoding pb_encoding.ContentEncoding) *Client {
	s := Client{
		mqttClient:  client,
		topicPrefix: topicPrefix,
		log:         zap.S().With("module", "rrpc.mqtt_pb_client"),
		rpcClientCodecPool: sync.Pool{
			New: func() interface{} {
				return NewRPCClientCodec(encoding)
			},
		},
	}

	client.EnsureConnected()

	return &s
}

// Client returns the MQTT client associated with the reverse RPC client.
func (s *Client) Client() mqtt_adapter.MQTTClientAdapter {
	return s.mqttClient
}

// OnConnect registers a callback function to be called when the MQTT client is connected.
// The callback function should have no arguments and no return value.
// It returns an integer value representing the registration ID.
func (s *Client) OnConnect(cb func()) int {
	return s.mqttClient.OnConnect(cb)
}

// IsConnected returns a boolean value indicating whether the MQTT client is currently connected.
func (s *Client) IsConnected() bool {
	return s.mqttClient.IsConnected()
}

// Close closes the MQTT client connection.
// It disconnects the MQTT client and returns any error encountered.
func (s *Client) Close() error {
	s.mqttClient.Disconnect()
	return nil
}

// Subscribe subscribes to a topic with the specified quality of service (QoS) level
// and registers a callback function to handle incoming messages.
// The topic parameter specifies the topic to subscribe to.
// The qos parameter specifies the desired QoS level for the subscription.
// The cb parameter is a callback function that will be called when a message is received on the subscribed topic.
func (s *Client) Subscribe(topic string, qos byte, cb mqtt_adapter.MessageCallback) {
	s.mqttClient.Subscribe(context.TODO(), topic, qos, cb)
}

func (s *Client) createRPCClient(deviceID string, encoding pb_encoding.ContentEncoding) (*rpc.Client, func(), error) {
	id := uuid.NewString()
	requestTopic := path.Join(s.topicPrefix, deviceID, "request", id)
	responseTopic := path.Join(s.topicPrefix, deviceID, "response", id)
	conn, err := NewConn(requestTopic, responseTopic, s.mqttClient, reverse_rpc.DefaultQoS)
	if err != nil {
		return nil, nil, err
	}

	codec := s.rpcClientCodecPool.Get().(*RPCClientCodec)
	codec.Reset(conn)
	codec.SetEncoding(encoding)

	rpcClient := rpc.NewClientWithCodec(codec)

	return rpcClient, func() {
		rpcClient.Close()
		s.rpcClientCodecPool.Put(codec)
	}, nil
}

type callOpt struct {
	encoding pb_encoding.ContentEncoding
}

// CallOption represents an option for making a remote procedure call.
type CallOption func(o *callOpt)

// WithEncoding sets the encoding for the RPC call.
// It takes a pb.ContentEncoding as a parameter and returns a CallOption.
// The encoding determines how the data is encoded before being sent over the network.
// Example usage:
//
//	opt := WithEncoding(pb.ContentEncoding_GZIP)
//	client.Call(ctx, method, request, response, opt)
func WithEncoding(encoding pb_encoding.ContentEncoding) CallOption {
	return func(o *callOpt) {
		o.encoding = encoding
	}
}

// Call invokes a remote procedure call (RPC) on the MQTT client.
// It sends the specified arguments to the specified service method
// and waits for the reply. The call options can be used to customize
// the behavior of the RPC call.
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

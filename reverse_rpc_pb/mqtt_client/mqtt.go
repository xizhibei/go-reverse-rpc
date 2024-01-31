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
	// ErrClientIsNotReady is an error indicating that the client is not ready.
	ErrClientIsNotReady = errors.New("[RRPC] client is not ready")
)

// Client represents an MQTT client used for reverse RPC communication.
type Client struct {
	mqttClient         *mqtt.Client
	log                *zap.SugaredLogger
	rpcClientCodecPool sync.Pool

	topicPrefix string
	qos         byte
}

// New creates a new MQTT client with the specified URI, client ID, topic prefix, and options.
// It returns a pointer to the created Client and an error if any.
func New(uri, clientID, topicPrefix string, options ...mqtt.Option) (*Client, error) {
	client, err := mqtt.NewClient(uri, clientID, options...)
	if err != nil {
		return nil, err
	}

	s := NewWithMQTTClient(topicPrefix, 0, pb.ContentEncoding_BROTLI, client)

	client.EnsureConnected()

	return s, nil
}

// NewWithMQTTClient creates a new instance of the Client struct with the provided parameters.
// It initializes the MQTT client, sets the topic prefix, quality of service (QoS), content encoding,
// and initializes the RPC client codec pool.
// The returned pointer to the Client struct can be used to interact with the MQTT client.
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

// Client returns the MQTT client associated with the reverse RPC client.
func (s *Client) Client() *mqtt.Client {
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

// CallOption represents an option for making a remote procedure call.
type CallOption func(o *callOpt)

// WithEncoding sets the encoding for the RPC call.
// It takes a pb.ContentEncoding as a parameter and returns a CallOption.
// The encoding determines how the data is encoded before being sent over the network.
// Example usage:
//
//	opt := WithEncoding(pb.ContentEncoding_JSON)
//	client.Call(ctx, method, request, response, opt)
func WithEncoding(encoding pb.ContentEncoding) CallOption {
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

package mqtt_json_client

import (
	"context"
	"net/rpc"
	"path"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/xizhibei/go-reverse-rpc/mqtt"
	"go.uber.org/zap"
)

var (
	ErrClientIsNotReady = errors.New("[RRPC] client is not reader")
)

type Client struct {
	mqttClient *mqtt.Client
	log        *zap.SugaredLogger

	topicPrefix string
	qos         byte
}

func New(uri, clientID, topicPrefix string, options ...mqtt.Option) (*Client, error) {
	client, err := mqtt.NewClient(uri, clientID, options...)
	if err != nil {
		return nil, err
	}

	s := NewWithMQTTClient(topicPrefix, 0, client)

	client.EnsureConnected()

	return s, nil
}

func NewWithMQTTClient(topicPrefix string, qos byte, client *mqtt.Client) *Client {
	s := Client{
		mqttClient:  client,
		qos:         qos,
		topicPrefix: topicPrefix,
		log:         zap.S().With("module", "reverse_rpc.mqtt"),
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

func (s *Client) createRPCClient(machineID string) (*rpc.Client, error) {
	id := uuid.NewString()
	requestTopic := path.Join(s.topicPrefix, machineID, "request", id)
	responseTopic := path.Join(s.topicPrefix, machineID, "response", id)
	client, err := Dial(requestTopic, responseTopic, s.mqttClient, s.qos)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func (s *Client) Call(ctx context.Context, machineID, serviceMethod string, args interface{}, reply interface{}) error {
	rpcClient, err := s.createRPCClient(machineID)
	if err != nil {
		return err
	}
	defer rpcClient.Close()

	call := rpcClient.Go(serviceMethod, args, reply, nil)

	select {
	case <-call.Done:
		return call.Error
	case <-ctx.Done():
		return ctx.Err()
	}
}

package mqtt_json_client

import (
	"context"
	"net/rpc"
	"path"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	rrpc "github.com/xizhibei/go-reverse-rpc"
	"github.com/xizhibei/go-reverse-rpc/mqtt_adapter"
	"go.uber.org/zap"
)

var (
	ErrClientIsNotReady = errors.New("[RRPC] client is not reader")
)

type Client struct {
	mqttClient mqtt_adapter.MQTTClientAdapter
	log        *zap.SugaredLogger

	topicPrefix string
}

func New(client mqtt_adapter.MQTTClientAdapter, topicPrefix string) *Client {
	s := Client{
		mqttClient:  client,
		topicPrefix: topicPrefix,
		log:         zap.S().With("module", "rrpc.mqtt_json_client"),
	}

	client.EnsureConnected()

	return &s
}

func (s *Client) Client() mqtt_adapter.MQTTClientAdapter {
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

func (s *Client) Subscribe(topic string, qos byte, cb mqtt_adapter.MessageCallback) {
	s.mqttClient.Subscribe(context.TODO(), topic, qos, cb)
}

func (s *Client) createRPCClient(targetId string) (*rpc.Client, error) {
	id := uuid.NewString()
	requestTopic := path.Join(s.topicPrefix, targetId, "request", id)
	responseTopic := path.Join(s.topicPrefix, targetId, "response", id)
	client, err := Dial(requestTopic, responseTopic, s.mqttClient, rrpc.DefaultQoS)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func (s *Client) Call(ctx context.Context, targetId, serviceMethod string, args interface{}, reply interface{}) error {
	rpcClient, err := s.createRPCClient(targetId)
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

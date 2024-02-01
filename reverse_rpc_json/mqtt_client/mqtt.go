package mqtt_json_client

import (
	"context"
	"net/rpc"
	"path"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	reverse_rpc "github.com/xizhibei/go-reverse-rpc"
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
}

func New(topicPrefix string, client *mqtt.Client) *Client {
	s := Client{
		mqttClient:  client,
		topicPrefix: topicPrefix,
		log:         zap.S().With("module", "rrpc.pb.mqtt.client"),
	}

	client.EnsureConnected()

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
	client, err := Dial(requestTopic, responseTopic, s.mqttClient, reverse_rpc.DefaultQoS)
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

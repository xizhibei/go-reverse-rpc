package mqttjson

import (
	"context"
	"net/rpc"
	"path"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	rrpc "github.com/xizhibei/go-reverse-rpc"
	"github.com/xizhibei/go-reverse-rpc/mqttadapter"
	"github.com/xizhibei/go-reverse-rpc/telemetry"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

var (
	ErrClientIsNotReady = errors.New("[RRPC] client is not reader")
)

type Client struct {
	mqttClient  mqttadapter.MQTTClientAdapter
	log         *zap.SugaredLogger
	telemetry   *telemetry.Telemetry
	topicPrefix string
}

func NewClient(client mqttadapter.MQTTClientAdapter, topicPrefix string) *Client {
	tel, _ := telemetry.NewNoop()

	s := Client{
		mqttClient:  client,
		topicPrefix: topicPrefix,
		telemetry:   tel,
		log:         zap.S().With("module", "rrpc.mqttjsonclient"),
	}

	client.EnsureConnected()

	return &s
}

func (s *Client) Client() mqttadapter.MQTTClientAdapter {
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

func (s *Client) Subscribe(topic string, qos byte, cb mqttadapter.MessageCallback) {
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

	var span trace.Span
	ctx, span = s.telemetry.StartSpan(ctx, "RRPC.Client.Call "+serviceMethod)
	defer span.End()

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

// SetTelemetry sets the telemetry for the server
func (s *Client) SetTelemetry(tel *telemetry.Telemetry) {
	s.telemetry = tel
}

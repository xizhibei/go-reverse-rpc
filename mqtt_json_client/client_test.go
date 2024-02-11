package mqtt_json_client_test

import (
	"context"
	"encoding/json"
	"path"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
	"github.com/xizhibei/go-reverse-rpc/json_encoding"
	"github.com/xizhibei/go-reverse-rpc/mqtt_adapter"
	mock_mqtt_adapter "github.com/xizhibei/go-reverse-rpc/mqtt_adapter/mock"
	"github.com/xizhibei/go-reverse-rpc/mqtt_json_client"
	"go.uber.org/mock/gomock"
)

type message struct {
	duplicate bool
	qos       byte
	retained  bool
	topic     string
	messageID uint16
	payload   []byte
	once      sync.Once
	ack       func()
}

func (m *message) Duplicate() bool {
	return m.duplicate
}

func (m *message) Qos() byte {
	return m.qos
}

func (m *message) Retained() bool {
	return m.retained
}

func (m *message) Topic() string {
	return m.topic
}

func (m *message) MessageID() uint16 {
	return m.messageID
}

func (m *message) Payload() []byte {
	return m.payload
}

func (m *message) Ack() {
	m.once.Do(m.ack)
}

type MQTTJsonClientTestSuite struct {
	suite.Suite
	client     *mqtt_json_client.Client
	mqttClient *mock_mqtt_adapter.MockMQTTClientAdapter

	topicPrefix string
	deviceId    string
}

func (suite *MQTTJsonClientTestSuite) SetupSuite() {
	suite.topicPrefix = "test/example"
	suite.deviceId = uuid.NewString()
	ctrl := gomock.NewController(suite.T())
	suite.mqttClient = mock_mqtt_adapter.NewMockMQTTClientAdapter(ctrl)
	suite.mqttClient.EXPECT().
		EnsureConnected()
	suite.client = mqtt_json_client.New(suite.mqttClient, suite.topicPrefix)
}

func (suite *MQTTJsonClientTestSuite) TestCall() {
	var onSubCallback mqtt_adapter.MessageCallback

	suite.mqttClient.EXPECT().
		Subscribe(
			gomock.Any(),
			gomock.Any(),
			gomock.Any(),
			gomock.Any(),
		).
		Do(func(_ context.Context, _ string, _ byte, cb mqtt_adapter.MessageCallback) {
			onSubCallback = cb
		})

	suite.mqttClient.EXPECT().
		OnConnect(gomock.Any()).
		DoAndReturn(func(cb func()) int {
			cb()
			return 0
		})

	suite.mqttClient.EXPECT().
		Unsubscribe(gomock.Any(), gomock.Any())

	params := map[string]string{
		"test": "test",
	}
	p, _ := json.Marshal(params)

	req := json_encoding.Request{
		ID:     0,
		Method: "test",
		Params: p,
		Status: 0,
	}

	// reqBytes, err := json.Marshal(req)
	// suite.NoError(err)

	suite.mqttClient.EXPECT().
		PublishBytes(
			gomock.Any(),
			gomock.Any(),
			gomock.Any(),
			gomock.Any(),
			gomock.Any(),
		)

	go func() {
		time.Sleep(200 * time.Millisecond)

		body := map[string]string{
			"test": "test",
		}
		b, _ := json.Marshal(body)

		res := json_encoding.Response{
			ID:     0,
			Method: "test",
			Data:   b,
			Status: 200,
		}
		resBytes, _ := json.Marshal(res)

		onSubCallback(suite.mqttClient, &message{
			payload: resBytes,
			topic:   path.Join(suite.topicPrefix, suite.deviceId, "response/test"),
		})
	}()

	ctx := context.Background()
	reply := map[string]string{}
	err := suite.client.Call(ctx, suite.deviceId, req.Method, params, &reply)
	suite.NoError(err)
}

func TestMQTTJson(t *testing.T) {
	suite.Run(t, new(MQTTJsonClientTestSuite))
}

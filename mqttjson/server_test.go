package mqttjson_test

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"sync"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
	rrpc "github.com/xizhibei/go-reverse-rpc"
	"github.com/xizhibei/go-reverse-rpc/mqttadapter"
	mock_mqttadapter "github.com/xizhibei/go-reverse-rpc/mqttadapter/mock"
	mock_mqtt_client "github.com/xizhibei/go-reverse-rpc/mqttadapter/mock/mqtt"
	"github.com/xizhibei/go-reverse-rpc/mqttjson"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

// type message struct {
// 	duplicate bool
// 	qos       byte
// 	retained  bool
// 	topic     string
// 	messageID uint16
// 	payload   []byte
// 	once      sync.Once
// 	ack       func()
// }

// func (m *message) Duplicate() bool {
// 	return m.duplicate
// }

// func (m *message) Qos() byte {
// 	return m.qos
// }

// func (m *message) Retained() bool {
// 	return m.retained
// }

// func (m *message) Topic() string {
// 	return m.topic
// }

// func (m *message) MessageID() uint16 {
// 	return m.messageID
// }

// func (m *message) Payload() []byte {
// 	return m.payload
// }

// func (m *message) Ack() {
// 	m.once.Do(m.ack)
// }

type ReqBody struct {
	A string `json:"a"`
	B int    `json:"b"`
}

type MQTTJsonServerTestSuite struct {
	suite.Suite
	server           *mqttjson.Server
	mqttClient       *mock_mqttadapter.MockMQTTClientAdapter
	originMqttClient *mock_mqtt_client.MockClient

	topicPrefix string
	deviceId    string
}

func (suite *MQTTJsonServerTestSuite) SetupSuite() {
	log, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	zap.ReplaceGlobals(log)

	suite.topicPrefix = "test/example"
	suite.deviceId = uuid.NewString()
	ctrl := gomock.NewController(suite.T())
	suite.mqttClient = mock_mqttadapter.NewMockMQTTClientAdapter(ctrl)
	suite.originMqttClient = mock_mqtt_client.NewMockClient(ctrl)
}

func (suite *MQTTJsonServerTestSuite) TestReceiveCall() {
	var onSubCallback mqttadapter.MessageCallback

	suite.mqttClient.EXPECT().
		Subscribe(
			gomock.Any(),
			gomock.Any(),
			gomock.Any(),
			gomock.Any(),
		).
		Do(func(_ context.Context, _ string, _ byte, cb mqttadapter.MessageCallback) {
			onSubCallback = cb
		})

	suite.mqttClient.EXPECT().
		OnConnect(gomock.Any()).
		DoAndReturn(func(cb func()) int {
			cb()
			return 0
		})

	suite.mqttClient.EXPECT().
		EnsureConnected()

	suite.server = mqttjson.NewServer(
		suite.mqttClient,
		path.Join(suite.topicPrefix, suite.deviceId, "request/+"),
		validator.New(),
	)

	suite.mqttClient.EXPECT().
		GetClientOptions().
		Return(mqtt.NewClientOptions().AddBroker("tcp://localhost:1883")).
		AnyTimes()

	suite.mqttClient.EXPECT().
		PublishBytes(
			gomock.Any(),
			gomock.Any(),
			gomock.Any(),
			gomock.Any(),
			gomock.Any(),
		).
		AnyTimes()

	reqBody := ReqBody{
		A: "123",
		B: 123,
	}

	b, err := json.Marshal(reqBody)
	suite.NoError(err)

	var wg sync.WaitGroup
	{
		suite.server.Register("test", &rrpc.Handler{
			Timeout: 5 * time.Second,
			Method: func(c rrpc.Context) {
				defer wg.Done()

				req := ReqBody{}
				err := c.Bind(&req)
				if err != nil {
					c.ReplyError(rrpc.RPCStatusClientError, err)
					return
				}

				suite.Equal(req.A, reqBody.A)
				suite.Equal(req.B, reqBody.B)

				c.ReplyOK(req)
			},
		})

		req := mqttjson.Request{
			ID:     0,
			Method: "test",
			Params: b,
		}
		reqBytes, err := json.Marshal(req)
		suite.NoError(err)

		wg.Add(1)
		onSubCallback(suite.mqttClient, &message{
			payload: reqBytes,
			topic:   path.Join(suite.topicPrefix, suite.deviceId, "request/1"),
		})
	}
	{
		suite.server.Register("test-fail", &rrpc.Handler{
			Timeout: 5 * time.Second,
			Method: func(c rrpc.Context) {
				defer wg.Done()

				req := ReqBody{}
				err := c.Bind(&req)
				if err != nil {
					c.ReplyError(rrpc.RPCStatusClientError, err)
					return
				}

				suite.Equal(req.A, reqBody.A)
				suite.Equal(req.B, reqBody.B)

				c.ReplyError(rrpc.RPCStatusServerError, fmt.Errorf("test error"))
			},
		})

		req := mqttjson.Request{
			ID:     0,
			Method: "test-fail",
			Params: b,
		}
		reqBytes, err := json.Marshal(req)
		suite.NoError(err)
		wg.Add(1)
		onSubCallback(suite.mqttClient, &message{
			payload: reqBytes,
			topic:   path.Join(suite.topicPrefix, suite.deviceId, "request/2"),
		})
	}
	{
		suite.server.Register("test-panic", &rrpc.Handler{
			Timeout: 5 * time.Second,
			Method: func(c rrpc.Context) {
				wg.Done()

				panic(fmt.Errorf("panic error"))
			},
		})

		req := mqttjson.Request{
			ID:     0,
			Method: "test-panic",
			Params: b,
		}
		reqBytes, err := json.Marshal(req)
		suite.NoError(err)
		wg.Add(1)
		onSubCallback(suite.mqttClient, &message{
			payload: reqBytes,
			topic:   path.Join(suite.topicPrefix, suite.deviceId, "request/3"),
		})
	}
	wg.Wait()

	suite.mqttClient.EXPECT().
		Disconnect()
	suite.server.Close()
}

func TestMQTTJsonServer(t *testing.T) {
	suite.Run(t, new(MQTTJsonServerTestSuite))
}

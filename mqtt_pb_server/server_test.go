package mqtt_pb_server_test

import (
	"context"
	"fmt"
	"path"
	"reflect"
	"sync"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
	reverse_rpc "github.com/xizhibei/go-reverse-rpc"
	"github.com/xizhibei/go-reverse-rpc/mqtt_adapter"
	mock_mqtt_adapter "github.com/xizhibei/go-reverse-rpc/mqtt_adapter/mock"
	mock_mqtt_client "github.com/xizhibei/go-reverse-rpc/mqtt_adapter/mock/mqtt"
	"github.com/xizhibei/go-reverse-rpc/mqtt_pb_server"
	"github.com/xizhibei/go-reverse-rpc/pb_encoding"
	testpb "github.com/xizhibei/go-reverse-rpc/pb_encoding/test"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
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

type MQTTPBServerTestSuite struct {
	suite.Suite
	server           *mqtt_pb_server.MqttServer
	mqttClient       *mock_mqtt_adapter.MockMQTTClientAdapter
	originMqttClient *mock_mqtt_client.MockClient

	topicPrefix string
	deviceId    string
}

func (suite *MQTTPBServerTestSuite) SetupSuite() {
	log, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	zap.ReplaceGlobals(log)

	suite.topicPrefix = "test/example"
	suite.deviceId = uuid.NewString()
	ctrl := gomock.NewController(suite.T())
	suite.mqttClient = mock_mqtt_adapter.NewMockMQTTClientAdapter(ctrl)
	suite.originMqttClient = mock_mqtt_client.NewMockClient(ctrl)
}

func (suite *MQTTPBServerTestSuite) TestReceiveCall() {
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
		EnsureConnected()

	suite.server = mqtt_pb_server.New(
		suite.mqttClient,
		path.Join(suite.topicPrefix, suite.deviceId, "request/+"),
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

	reqBody := testpb.TestRequestBody{
		Id:  123,
		Str: "123",
	}

	b, err := proto.Marshal(&reqBody)
	suite.NoError(err)

	var wg sync.WaitGroup
	{
		suite.server.Register("test", &reverse_rpc.Handler{
			Timeout: 5 * time.Second,
			Method: func(c reverse_rpc.Context) {
				defer wg.Done()

				req := testpb.TestRequestBody{}
				err := c.Bind(&req)
				if err != nil {
					c.ReplyError(reverse_rpc.RPCStatusClientError, err)
					return
				}

				suite.Equal(req.Id, reqBody.Id)
				suite.Equal(req.Str, reqBody.Str)

				c.ReplyOK(&req)
			},
		})

		req := pb_encoding.Request{
			Id:     0,
			Method: "test",
			Body: &anypb.Any{
				TypeUrl: reflect.TypeOf(&reqBody).String(),
				Value:   b,
			},
		}
		reqBytes, err := proto.Marshal(&req)
		suite.NoError(err)

		wg.Add(1)
		onSubCallback(suite.mqttClient, &message{
			payload: reqBytes,
			topic:   path.Join(suite.topicPrefix, suite.deviceId, "request/1"),
		})
	}
	{
		suite.server.Register("test-fail", &reverse_rpc.Handler{
			Timeout: 5 * time.Second,
			Method: func(c reverse_rpc.Context) {
				defer wg.Done()

				req := testpb.TestRequestBody{}
				err := c.Bind(&req)
				if err != nil {
					c.ReplyError(reverse_rpc.RPCStatusClientError, err)
					return
				}

				suite.Equal(req.Id, reqBody.Id)
				suite.Equal(req.Str, reqBody.Str)

				c.ReplyError(reverse_rpc.RPCStatusServerError, fmt.Errorf("test error"))
			},
		})

		req := pb_encoding.Request{
			Id:     0,
			Method: "test-fail",
			Body: &anypb.Any{
				TypeUrl: reflect.TypeOf(&reqBody).String(),
				Value:   b,
			},
		}
		reqBytes, err := proto.Marshal(&req)
		suite.NoError(err)

		wg.Add(1)
		onSubCallback(suite.mqttClient, &message{
			payload: reqBytes,
			topic:   path.Join(suite.topicPrefix, suite.deviceId, "request/2"),
		})
	}
	{
		suite.server.Register("test-panic", &reverse_rpc.Handler{
			Timeout: 5 * time.Second,
			Method: func(c reverse_rpc.Context) {
				wg.Done()

				panic(fmt.Errorf("panic error"))
			},
		})

		req := pb_encoding.Request{
			Id:     0,
			Method: "test-panic",
			Body: &anypb.Any{
				TypeUrl: reflect.TypeOf(&reqBody).String(),
				Value:   b,
			},
		}
		reqBytes, err := proto.Marshal(&req)
		suite.NoError(err)

		wg.Add(1)
		onSubCallback(suite.mqttClient, &message{
			payload: reqBytes,
			topic:   path.Join(suite.topicPrefix, suite.deviceId, "request/2"),
		})
	}
	wg.Wait()

	suite.mqttClient.EXPECT().
		Disconnect()
	suite.server.Close()
}

func TestMQTTPB(t *testing.T) {
	suite.Run(t, new(MQTTPBServerTestSuite))
}

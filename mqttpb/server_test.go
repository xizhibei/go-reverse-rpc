package mqttpb_test

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
	rrpc "github.com/xizhibei/go-reverse-rpc"
	"github.com/xizhibei/go-reverse-rpc/mqttadapter"
	mock_mqttadapter "github.com/xizhibei/go-reverse-rpc/mqttadapter/mock"
	mock_mqtt "github.com/xizhibei/go-reverse-rpc/mqttadapter/mock/mqtt"
	"github.com/xizhibei/go-reverse-rpc/mqttpb"
	testpb "github.com/xizhibei/go-reverse-rpc/mqttpb/test"
	"github.com/xizhibei/go-reverse-rpc/telemetry"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

func NewMockMessage(ctrl *gomock.Controller, payload []byte, topic string) mqttadapter.Message {
	m := mock_mqtt.NewMockMessage(ctrl)
	m.EXPECT().Payload().Return(payload).AnyTimes()
	m.EXPECT().Topic().Return(topic).AnyTimes()
	m.EXPECT().Retained().Return(false).AnyTimes()
	return m
}

type MQTTPBServerTestSuite struct {
	suite.Suite
	server           *mqttpb.Server
	mockCtrl         *gomock.Controller
	mqttClient       *mock_mqttadapter.MockMQTTClientAdapter
	originMqttClient *mock_mqtt.MockClient
	testTelemetry    *telemetry.TestTelemetry

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
	suite.mockCtrl = gomock.NewController(suite.T())
	suite.mqttClient = mock_mqttadapter.NewMockMQTTClientAdapter(suite.mockCtrl)
	suite.originMqttClient = mock_mqtt.NewMockClient(suite.mockCtrl)
	suite.testTelemetry = telemetry.NewTestTelemetry(suite.T())
}

func (suite *MQTTPBServerTestSuite) TearDownSuite() {
	if err := suite.testTelemetry.Shutdown(context.Background()); err != nil {
		suite.T().Errorf("Failed to shutdown telemetry: %v", err)
	}
}

func (suite *MQTTPBServerTestSuite) TestReceiveCall() {
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

	suite.server = mqttpb.NewServer(
		suite.mqttClient,
		suite.topicPrefix,
		suite.deviceId,
	)

	tel, err := telemetry.New(context.Background(), telemetry.Config{
		ServiceName:    "test-service",
		ServiceVersion: "v1.0.0",
		Environment:    "test",
	})
	suite.Require().NoError(err)
	suite.server.SetTelemetry(tel)
	suite.NoError(err)

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
		suite.server.Register("test", &rrpc.Handler{
			Timeout: 5 * time.Second,
			Method: func(c rrpc.Context) {
				defer wg.Done()

				req := testpb.TestRequestBody{}
				err := c.Bind(&req)
				if err != nil {
					c.ReplyError(rrpc.RPCStatusClientError, err)
					return
				}

				suite.Equal(req.Id, reqBody.Id)
				suite.Equal(req.Str, reqBody.Str)

				c.ReplyOK(&req)
			},
		})

		req := mqttpb.Request{
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
		onSubCallback(suite.mqttClient, NewMockMessage(suite.mockCtrl, reqBytes, path.Join(suite.topicPrefix, suite.deviceId, "request/1")))
	}
	{
		suite.server.Register("test-fail", &rrpc.Handler{
			Timeout: 5 * time.Second,
			Method: func(c rrpc.Context) {
				defer wg.Done()

				req := testpb.TestRequestBody{}
				err := c.Bind(&req)
				if err != nil {
					c.ReplyError(rrpc.RPCStatusClientError, err)
					return
				}

				suite.Equal(req.Id, reqBody.Id)
				suite.Equal(req.Str, reqBody.Str)

				c.ReplyError(rrpc.RPCStatusServerError, fmt.Errorf("test error"))
			},
		})

		req := mqttpb.Request{
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
		onSubCallback(suite.mqttClient, NewMockMessage(suite.mockCtrl, reqBytes, path.Join(suite.topicPrefix, suite.deviceId, "request/2")))
	}
	{
		suite.server.Register("test-panic", &rrpc.Handler{
			Timeout: 5 * time.Second,
			Method: func(c rrpc.Context) {
				wg.Done()

				panic(fmt.Errorf("panic error"))
			},
		})

		req := mqttpb.Request{
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
		onSubCallback(suite.mqttClient, NewMockMessage(suite.mockCtrl, reqBytes, path.Join(suite.topicPrefix, suite.deviceId, "request/3")))
	}
	wg.Wait()

	suite.mqttClient.EXPECT().
		Disconnect()
	suite.server.Close()
}

func TestMQTTPBServer(t *testing.T) {
	suite.Run(t, new(MQTTPBServerTestSuite))
}

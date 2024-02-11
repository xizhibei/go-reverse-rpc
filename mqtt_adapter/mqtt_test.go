package mqtt_adapter

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
	mock_mqtt_client "github.com/xizhibei/go-reverse-rpc/mqtt_adapter/mock/mqtt"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

type MQTTTestSuite struct {
	suite.Suite
	mockCtrl       *gomock.Controller
	mockMqttClient *mock_mqtt_client.MockClient
	mqttAdapter    *MQTTClientAdapterImpl

	topicPrefix string
	deviceId    string
}

func (suite *MQTTTestSuite) SetupSuite() {
	log, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	zap.ReplaceGlobals(log)

	suite.topicPrefix = "test/example"
	suite.deviceId = uuid.NewString()
	suite.mockCtrl = gomock.NewController(suite.T())
	suite.mockMqttClient = mock_mqtt_client.NewMockClient(suite.mockCtrl)

	mqttAdapter, err := New(
		"tcp://localhost:1883",
		uuid.NewString(),
		WithDebug(true),
	)
	suite.NoError(err)
	suite.mqttAdapter = mqttAdapter.(*MQTTClientAdapterImpl)
	suite.mqttAdapter.client.Disconnect(1000)
	suite.mqttAdapter.client = suite.mockMqttClient
}

func (suite *MQTTTestSuite) TearDownSuite() {
	suite.mockMqttClient.EXPECT().
		Disconnect(gomock.Any())
	suite.mqttAdapter.Disconnect()
}

func (suite *MQTTTestSuite) TestGetClient() {
	client := suite.mqttAdapter.GetMqttClient()
	suite.Equal(suite.mockMqttClient, client)
}

func (suite *MQTTTestSuite) TestGetClientOptions() {
	opts := suite.mqttAdapter.GetClientOptions()
	suite.Equal("tcp://localhost:1883", opts.Servers[0].String())
}

func (suite *MQTTTestSuite) TestOnConnect() {
	suite.mockMqttClient.EXPECT().
		IsConnected().
		Return(true)
	suite.mqttAdapter.OnConnect(func() {
		suite.T().Log("connected")
	})
	suite.Equal(1, len(suite.mqttAdapter.onConnectCallbaks))
}

func (suite *MQTTTestSuite) TestOffConnect() {
	suite.mqttAdapter.OffConnect(0)
	suite.Equal(0, len(suite.mqttAdapter.onConnectCallbaks))
}

func (suite *MQTTTestSuite) TestOnConnectLost() {
	suite.mqttAdapter.OnConnectLost(func(err error) {
		suite.T().Log("connect losted")
	})
	suite.Equal(1, len(suite.mqttAdapter.onConnectLostCallbaks))
}

func (suite *MQTTTestSuite) TestOffConnectLost() {
	suite.mqttAdapter.OffConnectLost(0)
	suite.Equal(0, len(suite.mqttAdapter.onConnectLostCallbaks))
}

func (suite *MQTTTestSuite) TestConnectAndWaitForSuccess() {
	suite.mockMqttClient.EXPECT().
		IsConnectionOpen().
		Return(false)
	suite.mockMqttClient.EXPECT().
		IsConnectionOpen().
		Return(false)

	token := mock_mqtt_client.NewMockToken(suite.mockCtrl)

	done := make(chan struct{})
	defer close(done)

	token.EXPECT().
		Done().
		Return(done)
	token.EXPECT().
		Error().
		Return(nil)

	suite.mockMqttClient.EXPECT().
		Connect().
		Return(token)

	go suite.mqttAdapter.ConnectAndWaitForSuccess()

	done <- struct{}{}
}

func (suite *MQTTTestSuite) TestSubscribe() {
	topic := "test"
	qos := byte(1)
	suite.mockMqttClient.EXPECT().
		Subscribe(gomock.Any(), gomock.Any(), gomock.Any())
	suite.mqttAdapter.
		Subscribe(context.Background(), topic, qos, func(ma MQTTClientAdapter, m Message) {})
}

func (suite *MQTTTestSuite) TestSubscribeWait() {
	token := mock_mqtt_client.NewMockToken(suite.mockCtrl)

	done := make(chan struct{})
	defer close(done)

	token.EXPECT().
		Done().
		Return(done)
	token.EXPECT().
		Error().
		Return(nil)

	topic := "test"
	qos := byte(1)
	suite.mockMqttClient.EXPECT().
		Subscribe(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(token)

	go func() {
		err := suite.mqttAdapter.SubscribeWait(context.Background(), topic, qos, func(ma MQTTClientAdapter, m Message) {})
		suite.NoError(err)
	}()

	done <- struct{}{}
}

func (suite *MQTTTestSuite) TestSubscribeMultiple() {
	filters := map[string]byte{
		"test": 1,
	}
	suite.mockMqttClient.EXPECT().
		SubscribeMultiple(gomock.Any(), gomock.Any())
	suite.mqttAdapter.SubscribeMultiple(context.Background(), filters, func(ma MQTTClientAdapter, m Message) {})
}

func (suite *MQTTTestSuite) TestSubscribeMultipleWait() {
	filters := map[string]byte{
		"test": 1,
	}
	token := mock_mqtt_client.NewMockToken(suite.mockCtrl)

	done := make(chan struct{})
	defer close(done)

	token.EXPECT().
		Done().
		Return(done)
	token.EXPECT().
		Error().
		Return(nil)

	suite.mockMqttClient.EXPECT().
		SubscribeMultiple(gomock.Any(), gomock.Any()).
		Return(token)

	go func() {
		err := suite.mqttAdapter.SubscribeMultipleWait(context.Background(), filters, func(ma MQTTClientAdapter, m Message) {})
		suite.NoError(err)
	}()

	done <- struct{}{}
}

func (suite *MQTTTestSuite) TestUnsubscribe() {
	suite.mockMqttClient.EXPECT().
		Unsubscribe(gomock.Any())

	topic := "test"
	suite.mqttAdapter.Unsubscribe(context.Background(), topic)
}

func (suite *MQTTTestSuite) TestUnsubscribeWait() {
	token := mock_mqtt_client.NewMockToken(suite.mockCtrl)

	done := make(chan struct{})
	defer close(done)

	token.EXPECT().
		Done().
		Return(done)
	token.EXPECT().
		Error().
		Return(nil)

	topic := "test"

	suite.mockMqttClient.EXPECT().
		Unsubscribe(gomock.Any()).
		Return(token)

	go func() {
		err := suite.mqttAdapter.UnsubscribeWait(context.Background(), topic)
		suite.NoError(err)
	}()

	done <- struct{}{}
}

func (suite *MQTTTestSuite) TestUnsubscribeAll() {
	suite.mockMqttClient.EXPECT().
		Unsubscribe(gomock.Any())
	suite.mqttAdapter.UnsubscribeAll(context.Background())
}

func (suite *MQTTTestSuite) TestUnsubscribeAllWait() {
	token := mock_mqtt_client.NewMockToken(suite.mockCtrl)

	done := make(chan struct{})
	defer close(done)

	token.EXPECT().
		Done().
		Return(done)
	token.EXPECT().
		Error().
		Return(nil)

	suite.mockMqttClient.EXPECT().
		Unsubscribe(gomock.Any()).
		Return(token)

	go func() {
		err := suite.mqttAdapter.UnsubscribeAllWait(context.Background())
		suite.NoError(err)
	}()

	done <- struct{}{}
}

func (suite *MQTTTestSuite) TestPublishBytes() {
	topic := "test"
	qos := byte(1)
	retained := true
	payload := []byte("test")
	suite.mockMqttClient.EXPECT().
		Publish(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
	suite.mqttAdapter.PublishBytes(context.Background(), topic, qos, retained, payload)
}

func (suite *MQTTTestSuite) TestPublishBytesWait() {
	token := mock_mqtt_client.NewMockToken(suite.mockCtrl)

	done := make(chan struct{})
	defer close(done)

	token.EXPECT().
		Done().
		Return(done)
	token.EXPECT().
		Error().
		Return(nil)

	topic := "test"
	qos := byte(1)
	retained := true
	payload := []byte("test")
	suite.mockMqttClient.EXPECT().
		Publish(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(token)

	go func() {
		err := suite.mqttAdapter.PublishBytesWait(context.Background(), topic, qos, retained, payload)
		suite.NoError(err)
	}()

	done <- struct{}{}
}

func (suite *MQTTTestSuite) TestPublishObject() {
	topic := "test"
	qos := byte(1)
	retained := true
	payload := []byte("test")
	suite.mockMqttClient.EXPECT().
		Publish(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
	err := suite.mqttAdapter.PublishObject(context.Background(), topic, qos, retained, payload)
	suite.NoError(err)
}

func (suite *MQTTTestSuite) TestPublishObjectWait() {
	token := mock_mqtt_client.NewMockToken(suite.mockCtrl)

	done := make(chan struct{})
	defer close(done)

	token.EXPECT().
		Done().
		Return(done)
	token.EXPECT().
		Error().
		Return(nil)

	topic := "test"
	qos := byte(1)
	retained := true
	payload := []byte("test")
	suite.mockMqttClient.EXPECT().
		Publish(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(token)

	go func() {
		err := suite.mqttAdapter.PublishObjectWait(context.Background(), topic, qos, retained, payload)
		suite.NoError(err)
	}()

	done <- struct{}{}
}

func TestMQTT(t *testing.T) {
	suite.Run(t, new(MQTTTestSuite))
}

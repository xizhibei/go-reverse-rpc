package mqttjson_test

import (
	"context"
	"encoding/json"
	"path"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
	"github.com/xizhibei/go-reverse-rpc/mqttadapter"
	mock_mqttadapter "github.com/xizhibei/go-reverse-rpc/mqttadapter/mock"
	"github.com/xizhibei/go-reverse-rpc/mqttjson"
	"go.uber.org/mock/gomock"
)

type MQTTJsonClientTestSuite struct {
	suite.Suite
	client     *mqttjson.Client
	mockCtrl   *gomock.Controller
	mqttClient *mock_mqttadapter.MockMQTTClientAdapter

	topicPrefix string
	deviceId    string
}

func (suite *MQTTJsonClientTestSuite) SetupSuite() {
	suite.topicPrefix = "test/example"
	suite.deviceId = uuid.NewString()
	suite.mockCtrl = gomock.NewController(suite.T())
	suite.mqttClient = mock_mqttadapter.NewMockMQTTClientAdapter(suite.mockCtrl)
	suite.mqttClient.EXPECT().
		EnsureConnected()
	suite.client = mqttjson.NewClient(suite.mqttClient, suite.topicPrefix)
}

func (suite *MQTTJsonClientTestSuite) TestCall() {
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
		Unsubscribe(gomock.Any(), gomock.Any())

	params := map[string]string{
		"test": "test",
	}
	p, _ := json.Marshal(params)

	req := mqttjson.Request{
		ID:     0,
		Method: "test",
		Params: p,
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

		res := mqttjson.Response{
			ID:     0,
			Method: "test",
			Data:   b,
			Status: 200,
		}
		resBytes, _ := json.Marshal(res)

		onSubCallback(suite.mqttClient, NewMockMessage(suite.mockCtrl, resBytes, path.Join(suite.topicPrefix, suite.deviceId, "request/test")))
	}()

	ctx := context.Background()
	reply := map[string]string{}
	err := suite.client.Call(ctx, suite.deviceId, req.Method, params, &reply)
	suite.NoError(err)
}

func TestMQTTJsonClient(t *testing.T) {
	suite.Run(t, new(MQTTJsonClientTestSuite))
}

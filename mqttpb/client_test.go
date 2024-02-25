package mqttpb_test

import (
	"context"
	"path"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
	"github.com/xizhibei/go-reverse-rpc/mqttadapter"
	mock_mqttadapter "github.com/xizhibei/go-reverse-rpc/mqttadapter/mock"
	"github.com/xizhibei/go-reverse-rpc/mqttpb"
	testpb "github.com/xizhibei/go-reverse-rpc/mqttpb/test"
	"go.uber.org/mock/gomock"
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

type MQTTPBClientTestSuite struct {
	suite.Suite
	client     *mqttpb.Client
	mqttClient *mock_mqttadapter.MockMQTTClientAdapter

	topicPrefix string
	deviceId    string
}

func (suite *MQTTPBClientTestSuite) SetupSuite() {
	suite.topicPrefix = "test/example"
	suite.deviceId = uuid.NewString()
	ctrl := gomock.NewController(suite.T())
	suite.mqttClient = mock_mqttadapter.NewMockMQTTClientAdapter(ctrl)
	suite.mqttClient.EXPECT().
		EnsureConnected()
	suite.client = mqttpb.NewClient(
		suite.mqttClient,
		suite.topicPrefix,
		mqttpb.ContentEncoding_GZIP,
	)
}

func (suite *MQTTPBClientTestSuite) TestCall() {
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

	params := testpb.TestRequestBody{
		Id:  123,
		Str: "1234",
	}

	b, err := proto.Marshal(&params)
	suite.NoError(err)

	req := mqttpb.Request{
		Id:     0,
		Method: "test",
		Body: &anypb.Any{
			TypeUrl: reflect.TypeOf(&params).String(),
			Value:   b,
		},
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

		body := testpb.TestResponseBody{
			Id:  456,
			Str: "456",
		}
		b, _ := proto.Marshal(&body)

		res := mqttpb.Response{
			Id:     0,
			Status: 200,
			Body: &anypb.Any{
				TypeUrl: reflect.TypeOf(&body).String(),
				Value:   b,
			},
		}
		resBytes, _ := proto.Marshal(&res)

		onSubCallback(suite.mqttClient, &message{
			payload: resBytes,
			topic:   path.Join(suite.topicPrefix, suite.deviceId, "response/test"),
		})
	}()

	ctx := context.Background()
	reply := testpb.TestResponseBody{}
	err = suite.client.Call(ctx, suite.deviceId, req.Method, &params, &reply)
	suite.NoError(err)
}

func TestMQTTPBClient(t *testing.T) {
	suite.Run(t, new(MQTTPBClientTestSuite))
}

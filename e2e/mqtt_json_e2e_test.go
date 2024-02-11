package mqtt_e2e_test

import (
	"context"
	"fmt"
	"os"
	"path"
	"testing"
	"time"

	reverse_rpc "github.com/xizhibei/go-reverse-rpc"
	"github.com/xizhibei/go-reverse-rpc/mqtt_adapter"
	"github.com/xizhibei/go-reverse-rpc/mqtt_json_client"
	"github.com/xizhibei/go-reverse-rpc/mqtt_json_server"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

type MQTTJsonTestSuite struct {
	suite.Suite
	service *mqtt_json_server.MqttServer
	client  *mqtt_json_client.Client

	topicPrefix string
	uri         string
	deviceId    string
}

func (suite *MQTTJsonTestSuite) SetupSuite() {
	log, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	zap.ReplaceGlobals(log)

	clientID := uuid.New().String()
	suite.deviceId = uuid.NewString()
	suite.uri = "tcp://test:123456@localhost:1883"
	suite.topicPrefix = "test/example"

	if uri := os.Getenv("MQTT_RRPC_TEST_URI"); uri != "" {
		suite.uri = uri
	}

	if topicPrefix := os.Getenv("MQTT_RRPC_TEST_TOPIC_PREFIX"); topicPrefix != "" {
		suite.topicPrefix = topicPrefix
	}

	suite.T().Logf("uri %s", suite.uri)
	suite.T().Logf("topicPrefix %s", suite.topicPrefix)

	iotServer, err := mqtt_adapter.New(suite.uri, clientID+"server")
	if err != nil {
		panic(err)
	}

	service := mqtt_json_server.New(
		iotServer,
		path.Join(suite.topicPrefix, suite.deviceId, "request/+"),
		validator.New(),
	)
	suite.service = service

	iotClient, err := mqtt_adapter.New(suite.uri, clientID+"client")
	if err != nil {
		panic(err)
	}

	client := mqtt_json_client.New(
		iotClient,
		suite.topicPrefix,
	)
	suite.client = client

	time.Sleep(500 * time.Millisecond)
}

func (suite *MQTTJsonTestSuite) TearDownSuite() {
	suite.service.Close()
	suite.client.Close()
}

type Req struct {
	A string `json:"a"`
	B int64  `json:"b"`
}

func (suite *MQTTJsonTestSuite) TestNormalCall() {
	reqParams := Req{
		A: "a",
		B: 1,
	}
	method := "test_normal_call"

	suite.service.Register(method, &reverse_rpc.Handler{
		Method: func(c reverse_rpc.Context) {
			var req Req
			err := c.Bind(&req)
			if err != nil {
				c.ReplyError(reverse_rpc.RPCStatusClientError, err)
				return
			}

			suite.Equal(req.A, reqParams.A)
			suite.Equal(req.B, reqParams.B)

			c.ReplyOK(req)
		},
		Timeout: 5 * time.Second,
	})

	var res Req
	err := suite.client.Call(context.Background(), suite.deviceId, method, &reqParams, &res)
	require.Nil(suite.T(), err)

	suite.Equal(res.A, reqParams.A)
	suite.Equal(res.B, reqParams.B)
}

func (suite *MQTTJsonTestSuite) TestErrCall() {
	reqParams := Req{
		A: "a",
		B: 1,
	}
	method := "test_error_call"

	suite.service.Register(method, &reverse_rpc.Handler{
		Method: func(c reverse_rpc.Context) {
			var req Req
			err := c.Bind(&req)
			if err != nil {
				c.ReplyError(reverse_rpc.RPCStatusClientError, err)
				return
			}

			suite.Equal(req.A, reqParams.A)
			suite.Equal(req.B, reqParams.B)

			c.ReplyError(reverse_rpc.RPCStatusClientError, fmt.Errorf("response error"))
		},
		Timeout: 5 * time.Second,
	})

	var res Req
	err := suite.client.Call(context.Background(), suite.deviceId, method, &reqParams, &res)
	suite.Equal("response error", err.Error())
}

func (suite *MQTTJsonTestSuite) TestTimeoutCall() {
	reqParams := Req{
		A: "a",
		B: 1,
	}
	method := "test_timeout_call"

	suite.service.Register(method, &reverse_rpc.Handler{
		Method: func(c reverse_rpc.Context) {
			var req Req
			err := c.Bind(&req)
			if err != nil {
				c.ReplyError(reverse_rpc.RPCStatusClientError, err)
				return
			}

			suite.Equal(req.A, reqParams.A)
			suite.Equal(req.B, reqParams.B)

			time.Sleep(100 * time.Millisecond)

			c.ReplyOK(req)
		},
		Timeout: 50 * time.Millisecond,
	})

	var res Req
	err := suite.client.Call(context.Background(), suite.deviceId, method, &reqParams, &res)
	suite.Equal("job request timed out", err.Error())
}

func (suite *MQTTJsonTestSuite) TestPanicCall() {
	reqParams := Req{
		A: "a",
		B: 1,
	}
	method := "test_panic_call"

	suite.service.Register(method, &reverse_rpc.Handler{
		Method: func(c reverse_rpc.Context) {
			var req Req
			err := c.Bind(&req)
			if err != nil {
				c.ReplyError(reverse_rpc.RPCStatusClientError, err)
				return
			}

			suite.Equal(req.A, reqParams.A)
			suite.Equal(req.B, reqParams.B)

			panic(fmt.Errorf("panic error"))
		},
		Timeout: 5 * time.Second,
	})

	var res Req
	err := suite.client.Call(context.Background(), suite.deviceId, method, &reqParams, &res)
	suite.Equal("panic in method test_panic_call panic error", err.Error())
}

func TestMQTTJson(t *testing.T) {
	suite.Run(t, new(MQTTJsonTestSuite))
}

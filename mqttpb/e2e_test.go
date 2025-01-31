package mqttpb_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	rrpc "github.com/xizhibei/go-reverse-rpc"
	"github.com/xizhibei/go-reverse-rpc/mqttadapter"
	"github.com/xizhibei/go-reverse-rpc/mqttpb"
	testpb "github.com/xizhibei/go-reverse-rpc/mqttpb/test"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

type MQTTPBTestSuite struct {
	suite.Suite
	service *mqttpb.Server
	client  *mqttpb.Client

	topicPrefix string
	uri         string
	deviceId    string
}

func (suite *MQTTPBTestSuite) SetupSuite() {
	log, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	zap.ReplaceGlobals(log)

	clientID := uuid.NewString()
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

	iotServer, err := mqttadapter.New(suite.uri, clientID+"server")
	if err != nil {
		panic(err)
	}

	service := mqttpb.NewServer(
		iotServer,
		suite.topicPrefix,
		suite.deviceId,
		rrpc.WithLimiter(1, 100),
	)
	suite.service = service

	iotClient, err := mqttadapter.New(suite.uri, clientID+"client")
	if err != nil {
		panic(err)
	}

	client := mqttpb.NewClient(
		iotClient,
		suite.topicPrefix,
		mqttpb.ContentEncoding_GZIP,
	)
	suite.client = client

	time.Sleep(500 * time.Millisecond)
}

func (suite *MQTTPBTestSuite) TearDownSuite() {
	suite.service.Close()
	suite.client.Close()
}

func (suite *MQTTPBTestSuite) TestNormalCall() {
	reqParams := testpb.TestRequestBody{
		Id:  123,
		Str: "testpb.TestRequestBody",
	}
	resParams := testpb.TestResponseBody{
		Id:  456,
		Str: "testpb.TestResponseBody",
	}

	method := "test_normal_call"

	suite.service.Register(method, &rrpc.Handler{
		Method: func(c rrpc.Context) {
			var req testpb.TestRequestBody
			err := c.Bind(&req)
			if err != nil {
				c.ReplyError(rrpc.RPCStatusClientError, err)
				return
			}

			if eg, ok := c.(mqttpb.EncodingGetter); ok {
				fmt.Printf("req encoding %v\n", eg.Encoding())
			}

			fmt.Printf("req %v", &req)

			suite.Equal(req.Id, reqParams.Id)
			suite.Equal(req.Str, reqParams.Str)

			c.ReplyOK(&resParams)
		},
		Timeout: 5 * time.Second,
	})

	encodings := []mqttpb.ContentEncoding{
		mqttpb.ContentEncoding_BROTLI,
		mqttpb.ContentEncoding_DEFLATE,
		mqttpb.ContentEncoding_GZIP,
		mqttpb.ContentEncoding_PLAIN,
	}

	for _, e := range encodings {
		var res testpb.TestResponseBody
		err := suite.client.Call(context.Background(), suite.deviceId, method, &reqParams, &res, mqttpb.WithEncoding(e))
		require.Nil(suite.T(), err)

		suite.Equal(res.Id, resParams.Id)
		suite.Equal(res.Str, resParams.Str)
	}
}

func (suite *MQTTPBTestSuite) TestErrCall() {
	reqParams := testpb.TestRequestBody{
		Id:  123,
		Str: "testpb.TestRequestBody",
	}

	method := "test_err_call"

	suite.service.Register(method, &rrpc.Handler{
		Method: func(c rrpc.Context) {
			var req testpb.TestRequestBody
			err := c.Bind(&req)
			if err != nil {
				c.ReplyError(rrpc.RPCStatusClientError, err)
				return
			}

			suite.Equal(req.Id, reqParams.Id)
			suite.Equal(req.Str, reqParams.Str)

			c.ReplyError(rrpc.RPCStatusClientError, fmt.Errorf("response error"))
		},
		Timeout: 5 * time.Second,
	})

	var res testpb.TestResponseBody
	err := suite.client.Call(context.Background(), suite.deviceId, method, &reqParams, &res)
	suite.Equal("response error", err.Error())
}

func (suite *MQTTPBTestSuite) TestTimeoutCall() {
	reqParams := testpb.TestRequestBody{
		Id:  123,
		Str: "testpb.TestRequestBody",
	}

	method := "test_timeout_call"

	suite.service.Register(method, &rrpc.Handler{
		Method: func(c rrpc.Context) {
			var req testpb.TestRequestBody
			err := c.Bind(&req)
			if err != nil {
				c.ReplyError(rrpc.RPCStatusClientError, err)
				return
			}

			suite.Equal(req.Id, reqParams.Id)
			suite.Equal(req.Str, reqParams.Str)

			time.Sleep(100 * time.Millisecond)

			c.ReplyOK(&req)
		},
		Timeout: 50 * time.Millisecond,
	})

	var res testpb.TestRequestBody
	err := suite.client.Call(context.Background(), suite.deviceId, method, &reqParams, &res)
	suite.Equal("job request timed out", err.Error())
}

func (suite *MQTTPBTestSuite) TestPanicCall() {
	reqParams := testpb.TestRequestBody{
		Id:  123,
		Str: "testpb.TestRequestBody",
	}

	method := "test_panic_call"

	suite.service.Register(method, &rrpc.Handler{
		Method: func(c rrpc.Context) {
			var req testpb.TestRequestBody
			err := c.Bind(&req)
			if err != nil {
				c.ReplyError(rrpc.RPCStatusClientError, err)
				return
			}

			suite.Equal(req.Id, reqParams.Id)
			suite.Equal(req.Str, reqParams.Str)

			panic(fmt.Errorf("panic error"))
		},
		Timeout: 5 * time.Second,
	})

	var res testpb.TestRequestBody
	err := suite.client.Call(context.Background(), suite.deviceId, method, &reqParams, &res)
	suite.Equal("panic in method test_panic_call panic error", err.Error())
}

func TestMQTTPBT(t *testing.T) {
	suite.Run(t, new(MQTTPBTestSuite))
}

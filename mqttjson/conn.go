package mqttjson

import (
	"bytes"
	"context"
	"io"
	"net/rpc"

	"github.com/xizhibei/go-reverse-rpc/mqttadapter"
	"go.uber.org/zap"
)

type rpcConn struct {
	readyChan chan struct{}
	ready     bool

	requestTopic  string
	responseTopic string
	c             mqttadapter.MQTTClientAdapter
	qos           byte

	log *zap.SugaredLogger

	bytesReader *bytes.Reader
}

func newRPCConn(requestTopic, responseTopic string, c mqttadapter.MQTTClientAdapter, qos byte) (io.ReadWriteCloser, error) {
	conn := rpcConn{
		readyChan:     make(chan struct{}),
		requestTopic:  requestTopic,
		responseTopic: responseTopic,
		c:             c,
		qos:           qos,
		log:           zap.S().With("module", "rrpc.mqttjsonclient.conn"),
	}

	c.OnConnect(func() {
		c.Subscribe(context.TODO(), responseTopic, qos, func(c mqttadapter.MQTTClientAdapter, m mqttadapter.Message) {
			conn.log.Infof("Receive data from %s len=%d", m.Topic(), len(m.Payload()))
			conn.bytesReader = bytes.NewReader(m.Payload())
			conn.ready = true
			conn.readyChan <- struct{}{}
		})
	})

	return &conn, nil
}

// Read reads data from the connection into the provided byte slice.
// It returns the number of bytes read and an error, if any.
// If the connection is not ready, it waits until it becomes ready.
// If the connection is closed, it returns 0 and io.EOF.
func (c *rpcConn) Read(data []byte) (int, error) {
	if c.ready {
		return c.bytesReader.Read(data)
	}
	_, ok := <-c.readyChan
	if !ok {
		return 0, io.EOF
	}
	return c.bytesReader.Read(data)
}

// Write writes the given data to the MQTT connection.
// It logs the information about the data being sent, including the topic and length.
// It then publishes the data to the request topic using the MQTT client.
// Finally, it returns the length of the data written and a nil error.
func (c *rpcConn) Write(data []byte) (int, error) {
	c.log.Infof("Send data to %s len=%d", c.requestTopic, len(data))
	c.c.PublishBytes(context.TODO(), c.requestTopic, c.qos, false, data)
	return len(data), nil
}

// Close closes the RPC connection and unsubscribes from the response topic.
// It returns an error if there was a problem unsubscribing from the topic.
func (c *rpcConn) Close() error {
	close(c.readyChan)
	c.c.Unsubscribe(context.Background(), c.responseTopic)
	return nil
}

// Dial establishes a connection to the MQTT broker and returns an RPC client.
// It takes the request topic, reply topic, MQTT client, and quality of service (QoS) as parameters.
// The function creates a new RPC connection using the provided parameters and returns a reverserpc_json.Client.
// If an error occurs during the connection establishment, it returns nil and the error.
func Dial(reqTopic, replyTopic string, c mqttadapter.MQTTClientAdapter, qos byte) (*rpc.Client, error) {
	conn, err := newRPCConn(reqTopic, replyTopic, c, qos)
	if err != nil {
		return nil, err
	}
	return NewRPCClient(conn), nil
}

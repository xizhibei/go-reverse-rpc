package mqtt_pb_client

import (
	"bytes"
	"io"
	"net/rpc"

	"github.com/xizhibei/go-reverse-rpc/mqtt"
	"github.com/xizhibei/go-reverse-rpc/reverse_rpc_pb"
	"go.uber.org/zap"
)

type rpcConn struct {
	readyChan   chan struct{}
	ready       bool
	bytesReader *bytes.Reader

	requestTopic  string
	responseTopic string
	c             *mqtt.Client
	qos           byte

	log *zap.SugaredLogger
}

// NewConn creates a new connection for reverse RPC over MQTT.
// It takes the request topic, response topic, MQTT client, and quality of service (QoS) as parameters.
// It returns an io.ReadWriteCloser and an error.
func NewConn(requestTopic, responseTopic string, c *mqtt.Client, qos byte) (io.ReadWriteCloser, error) {
	conn := rpcConn{
		readyChan:     make(chan struct{}),
		requestTopic:  requestTopic,
		responseTopic: responseTopic,
		c:             c,
		qos:           qos,
		log:           zap.S().With("module", "reverse_rpc.mqtt.conn"),
	}

	c.OnConnect(func() {
		tk := c.Subscribe(responseTopic, qos, func(c *mqtt.Client, m mqtt.Message) {
			conn.log.Infof("Receive data from %s len=%d", m.Topic(), len(m.Payload()))
			conn.bytesReader = bytes.NewReader(m.Payload())
			conn.ready = true
			conn.readyChan <- struct{}{}
		})
		if tk.Error() != nil {
			conn.log.Errorf("%v", tk.Error())
		}
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

// Write writes the given data to the MQTT broker.
// It logs the information about the data being sent, including the topic and length of the data.
// It then publishes the data to the request topic using the MQTT client.
// The number of bytes written and a nil error are returned.
func (c *rpcConn) Write(data []byte) (int, error) {
	c.log.Infof("Send data to %s len=%d", c.requestTopic, len(data))
	_ = c.c.PublishBytes(c.requestTopic, c.qos, false, data)
	return len(data), nil
}

// Close closes the RPC connection.
// It closes the ready channel and unsubscribes from the response topic.
// Returns an error if there was a problem unsubscribing from the topic.
func (c *rpcConn) Close() error {
	close(c.readyChan)
	return c.c.Unsubscribe(c.responseTopic)
}

// Dial establishes a connection to the MQTT broker and returns a new RPC client.
// It takes the request topic, reply topic, MQTT client, and quality of service (QoS) as parameters.
// The function creates a new connection using NewConn and returns a new RPC client using reverse_rpc_pb.NewClient.
// If an error occurs during the connection establishment, it is returned along with a nil client.
func Dial(reqTopic, replyTopic string, c *mqtt.Client, qos byte) (*rpc.Client, error) {
	conn, err := NewConn(reqTopic, replyTopic, c, qos)
	if err != nil {
		return nil, err
	}
	return reverse_rpc_pb.NewClient(conn), nil
}

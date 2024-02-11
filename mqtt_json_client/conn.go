package mqtt_json_client

import (
	"bytes"
	"io"
	"net/rpc"

	"github.com/xizhibei/go-reverse-rpc/mqtt_adapter"
	"go.uber.org/zap"
)

type rpcConn struct {
	readyChan chan struct{}
	ready     bool

	requestTopic  string
	responseTopic string
	c             mqtt_adapter.MQTTClientAdapter
	qos           byte

	log *zap.SugaredLogger

	bytesReader *bytes.Reader
}

func newRPCConn(requestTopic, responseTopic string, c mqtt_adapter.MQTTClientAdapter, qos byte) (io.ReadWriteCloser, error) {
	conn := rpcConn{
		readyChan:     make(chan struct{}),
		requestTopic:  requestTopic,
		responseTopic: responseTopic,
		c:             c,
		qos:           qos,
		log:           zap.S().With("module", "rrpc.mqtt_json_client.conn"),
	}

	c.OnConnect(func() {
		tk := c.Subscribe(responseTopic, qos, func(c mqtt_adapter.MQTTClientAdapter, m mqtt_adapter.Message) {
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

// Write writes the given data to the MQTT connection.
// It logs the information about the data being sent, including the topic and length.
// It then publishes the data to the request topic using the MQTT client.
// Finally, it returns the length of the data written and a nil error.
func (c *rpcConn) Write(data []byte) (int, error) {
	c.log.Infof("Send data to %s len=%d", c.requestTopic, len(data))
	_ = c.c.PublishBytes(c.requestTopic, c.qos, false, data)
	return len(data), nil
}

// Close closes the RPC connection and unsubscribes from the response topic.
// It returns an error if there was a problem unsubscribing from the topic.
func (c *rpcConn) Close() error {
	close(c.readyChan)
	return c.c.Unsubscribe(c.responseTopic)
}

// Dial establishes a connection to the MQTT broker and returns an RPC client.
// It takes the request topic, reply topic, MQTT client, and quality of service (QoS) as parameters.
// The function creates a new RPC connection using the provided parameters and returns a reverse_rpc_json.Client.
// If an error occurs during the connection establishment, it returns nil and the error.
func Dial(reqTopic, replyTopic string, c mqtt_adapter.MQTTClientAdapter, qos byte) (*rpc.Client, error) {
	conn, err := newRPCConn(reqTopic, replyTopic, c, qos)
	if err != nil {
		return nil, err
	}
	return NewClient(conn), nil
}

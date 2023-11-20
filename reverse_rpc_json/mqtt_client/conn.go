package mqtt_json_client

import (
	"bytes"
	"io"
	"net/rpc"

	"github.com/xizhibei/go-reverse-rpc/mqtt"
	"github.com/xizhibei/go-reverse-rpc/reverse_rpc_json"
	"go.uber.org/zap"
)

type rpcConn struct {
	readyChan chan struct{}
	ready     bool

	requestTopic  string
	responseTopic string
	c             *mqtt.Client
	qos           byte

	log *zap.SugaredLogger

	bytesReader *bytes.Reader
}

func newRPCConn(requestTopic, responseTopic string, c *mqtt.Client, qos byte) (io.ReadWriteCloser, error) {
	conn := rpcConn{
		readyChan:     make(chan struct{}),
		requestTopic:  requestTopic,
		responseTopic: responseTopic,
		c:             c,
		qos:           qos,
		log:           zap.S().With("module", "reverse_rpc.mqtt_client.conn"),
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

func (c *rpcConn) Write(data []byte) (int, error) {
	c.log.Infof("Send data to %s len=%d", c.requestTopic, len(data))
	_ = c.c.PublishBytes(c.requestTopic, c.qos, false, data)
	return len(data), nil
}

func (c *rpcConn) Close() error {
	close(c.readyChan)
	return c.c.Unsubscribe(c.responseTopic)
}

func Dial(reqTopic, replyTopic string, c *mqtt.Client, qos byte) (*rpc.Client, error) {
	conn, err := newRPCConn(reqTopic, replyTopic, c, qos)
	if err != nil {
		return nil, err
	}
	return reverse_rpc_json.NewClient(conn), nil
}

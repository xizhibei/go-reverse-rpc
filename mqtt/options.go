package mqtt

import (
	"crypto/tls"
	"fmt"
	"math/rand"
	"os"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type ClientOptions struct {
	*mqtt.ClientOptions
	enableStatus  bool
	enableDebug   bool
	onlineTopic   string
	onlinePayload []byte
}

type Option func(o *ClientOptions)

func WithDebug(debug bool) Option {
	return func(o *ClientOptions) {
		o.enableDebug = debug
	}
}

func WithUserPass(user, pass string) Option {
	return func(o *ClientOptions) {
		o.SetUsername(user)
		o.SetPassword(pass)
	}
}

func WithClientID(clientID string) Option {
	return func(o *ClientOptions) {
		o.SetClientID(clientID)
	}
}

func WithKeepAlive(keepalive time.Duration) Option {
	return func(o *ClientOptions) {
		o.SetKeepAlive(keepalive)
	}
}

func WithConnectRetryInterval(duration time.Duration) Option {
	return func(o *ClientOptions) {
		o.SetConnectRetry(true)
		o.SetConnectRetryInterval(duration)
	}
}

func WithProtocolVersion(pv uint) Option {
	return func(o *ClientOptions) {
		o.SetProtocolVersion(pv)
	}
}

func WithStore(store mqtt.Store) Option {
	return func(o *ClientOptions) {
		o.SetStore(store)
	}
}

func WithTlsConfig(cfg *tls.Config) Option {
	return func(o *ClientOptions) {
		o.SetTLSConfig(cfg)
	}
}

func WithWill(topic string, payload string, qos byte, retained bool) Option {
	return func(o *ClientOptions) {
		o.SetWill(topic, payload, qos, retained)
	}
}

func WithOfflineWill(topic string, payload string) Option {
	return func(o *ClientOptions) {
		o.SetWill(topic, payload, 1, true)
	}
}

func WithOnlineStatus(topic string, payload []byte) Option {
	return func(o *ClientOptions) {
		o.onlineTopic = topic
		o.onlinePayload = payload
	}
}

func WithStatus(
	onlineTopic string, onlinePayload []byte,
	offlineTopic string, offlinePayload []byte,
) Option {
	return func(o *ClientOptions) {
		o.enableStatus = true
		o.onlineTopic = onlineTopic
		o.onlinePayload = onlinePayload
		o.SetBinaryWill(offlineTopic, offlinePayload, 1, true)
	}
}

func WithFileStore(
	tempDir string,
) Option {
	return func(o *ClientOptions) {
		if tempDir == "" {
			tempDir = fmt.Sprintf("%d", rand.Intn(100000))
		}
		o.SetStore(mqtt.NewFileStore(os.TempDir() + "/" + tempDir))
	}
}

func WithQos(qos byte) Option {
	return func(o *ClientOptions) {
		o.WillQos = qos
	}
}

func WithMaxReconnectInterval(interval time.Duration) Option {
	return func(o *ClientOptions) {
		o.SetMaxReconnectInterval(interval)
	}
}

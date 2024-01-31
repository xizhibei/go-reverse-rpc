// Package mqtt provides options for configuring MQTT client.
// It includes various options for setting up the client, such as enabling debug mode,
// setting username and password, configuring TLS, setting will messages, and more.
package mqtt

import (
	"crypto/tls"
	"fmt"
	"math/rand"
	"os"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// ClientOptions represents the options for configuring the MQTT client.
type ClientOptions struct {
	*mqtt.ClientOptions
	enableStatus  bool
	enableDebug   bool
	onlineTopic   string
	onlinePayload []byte
}

// Option is a function that modifies the ClientOptions.
type Option func(o *ClientOptions)

// WithDebug sets the debug mode for the MQTT client.
func WithDebug(debug bool) Option {
	return func(o *ClientOptions) {
		o.enableDebug = debug
	}
}

// WithUserPass sets the username and password for the MQTT client.
func WithUserPass(user, pass string) Option {
	return func(o *ClientOptions) {
		o.SetUsername(user)
		o.SetPassword(pass)
	}
}

// WithClientID sets the client ID for the MQTT client.
func WithClientID(clientID string) Option {
	return func(o *ClientOptions) {
		o.SetClientID(clientID)
	}
}

// WithKeepAlive sets the keep alive interval for the MQTT client.
func WithKeepAlive(keepalive time.Duration) Option {
	return func(o *ClientOptions) {
		o.SetKeepAlive(keepalive)
	}
}

// WithConnectRetryInterval sets the interval for reconnecting to the MQTT broker.
func WithConnectRetryInterval(duration time.Duration) Option {
	return func(o *ClientOptions) {
		o.SetConnectRetry(true)
		o.SetConnectRetryInterval(duration)
	}
}

// WithProtocolVersion sets the MQTT protocol version for the client.
func WithProtocolVersion(pv uint) Option {
	return func(o *ClientOptions) {
		o.SetProtocolVersion(pv)
	}
}

// WithStore sets the store for the MQTT client.
func WithStore(store mqtt.Store) Option {
	return func(o *ClientOptions) {
		o.SetStore(store)
	}
}

// WithTlsConfig sets the TLS configuration for the MQTT client.
func WithTlsConfig(cfg *tls.Config) Option {
	return func(o *ClientOptions) {
		o.SetTLSConfig(cfg)
	}
}

// WithWill sets the will message for the MQTT client.
func WithWill(topic string, payload string, qos byte, retained bool) Option {
	return func(o *ClientOptions) {
		o.SetWill(topic, payload, qos, retained)
	}
}

// WithOfflineWill sets the offline will message for the MQTT client.
func WithOfflineWill(topic string, payload string) Option {
	return func(o *ClientOptions) {
		o.SetWill(topic, payload, 1, true)
	}
}

// WithOnlineStatus sets the online status topic and payload for the MQTT client.
func WithOnlineStatus(topic string, payload []byte) Option {
	return func(o *ClientOptions) {
		o.onlineTopic = topic
		o.onlinePayload = payload
	}
}

// WithStatus sets the online and offline status topics and payloads for the MQTT client.
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

// WithFileStore sets the file store for the MQTT client.
func WithFileStore(tempDir string) Option {
	return func(o *ClientOptions) {
		if tempDir == "" {
			tempDir = fmt.Sprintf("%d", rand.Intn(100000))
		}
		o.SetStore(mqtt.NewFileStore(os.TempDir() + "/" + tempDir))
	}
}

// WithQos sets the QoS level for the MQTT client.
func WithQos(qos byte) Option {
	return func(o *ClientOptions) {
		o.WillQos = qos
	}
}

// WithMaxReconnectInterval sets the maximum interval for reconnecting to the MQTT broker.
func WithMaxReconnectInterval(interval time.Duration) Option {
	return func(o *ClientOptions) {
		o.SetMaxReconnectInterval(interval)
	}
}

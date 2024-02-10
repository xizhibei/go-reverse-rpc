package mqtt_adapter

import (
	"crypto/tls"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/stretchr/testify/assert"
)

func TestWithDebug(t *testing.T) {
	options := &ClientOptions{ClientOptions: mqtt.NewClientOptions()}
	WithDebug(true)(options)
	assert.True(t, options.enableDebug)
}

func TestWithUserPass(t *testing.T) {
	options := &ClientOptions{ClientOptions: mqtt.NewClientOptions()}
	WithUserPass("username", "password")(options)
	assert.Equal(t, "username", options.Username)
	assert.Equal(t, "password", options.Password)
}

func TestWithClientID(t *testing.T) {
	options := &ClientOptions{ClientOptions: mqtt.NewClientOptions()}
	WithClientID("clientID")(options)
	assert.Equal(t, "clientID", options.ClientID)
}

func TestWithKeepAlive(t *testing.T) {
	options := &ClientOptions{ClientOptions: mqtt.NewClientOptions()}
	WithKeepAlive(30 * time.Second)(options)
	assert.Equal(t, int64(30), options.KeepAlive)
}

func TestWithConnectRetryInterval(t *testing.T) {
	options := &ClientOptions{ClientOptions: mqtt.NewClientOptions()}
	WithConnectRetryInterval(5 * time.Second)(options)
	assert.True(t, options.ConnectRetry)
	assert.Equal(t, 5*time.Second, options.ConnectRetryInterval)
}

func TestWithProtocolVersion(t *testing.T) {
	options := &ClientOptions{ClientOptions: mqtt.NewClientOptions()}
	WithProtocolVersion(4)(options)
	assert.Equal(t, uint(4), options.ProtocolVersion)
}

func TestWithStore(t *testing.T) {
	options := &ClientOptions{ClientOptions: mqtt.NewClientOptions()}
	store := mqtt.NewMemoryStore()
	WithStore(store)(options)
	assert.Equal(t, store, options.Store)
}

func TestWithTlsConfig(t *testing.T) {
	options := &ClientOptions{ClientOptions: mqtt.NewClientOptions()}
	tlsConfig := &tls.Config{}
	WithTlsConfig(tlsConfig)(options)
	assert.Equal(t, tlsConfig, options.TLSConfig)
}

func TestWithWill(t *testing.T) {
	options := &ClientOptions{ClientOptions: mqtt.NewClientOptions()}
	WithWill("topic", "payload", 1, true)(options)
	assert.Equal(t, "topic", string(options.WillTopic))
	assert.Equal(t, "payload", string(options.WillPayload))
	assert.Equal(t, byte(1), options.WillQos)
	assert.True(t, options.WillRetained)
}

func TestWithOfflineWill(t *testing.T) {
	options := &ClientOptions{ClientOptions: mqtt.NewClientOptions()}
	WithOfflineWill("topic", "payload")(options)
	assert.Equal(t, "topic", string(options.WillTopic))
	assert.Equal(t, "payload", string(options.WillPayload))
	assert.Equal(t, byte(1), options.WillQos)
	assert.True(t, options.WillRetained)
}

func TestWithOnlineStatus(t *testing.T) {
	options := &ClientOptions{ClientOptions: mqtt.NewClientOptions()}
	WithOnlineStatus("topic", []byte("payload"))(options)
	assert.Equal(t, "topic", options.onlineTopic)
	assert.Equal(t, []byte("payload"), options.onlinePayload)
}

func TestWithStatus(t *testing.T) {
	options := &ClientOptions{ClientOptions: mqtt.NewClientOptions()}
	WithStatus("onlineTopic", []byte("onlinePayload"), "offlineTopic", []byte("offlinePayload"))(options)
	assert.True(t, options.enableStatus)
	assert.Equal(t, "onlineTopic", options.onlineTopic)
	assert.Equal(t, []byte("onlinePayload"), options.onlinePayload)
	assert.Equal(t, "offlineTopic", options.WillTopic)
	assert.Equal(t, []byte("offlinePayload"), options.WillPayload)
	assert.Equal(t, byte(1), options.WillQos)
	assert.True(t, options.WillRetained)
}

func TestWithFileStore(t *testing.T) {
	options := &ClientOptions{ClientOptions: mqtt.NewClientOptions()}
	WithFileStore("tempDir")(options)
	assert.NotNil(t, options.Store)
}

func TestWithQos(t *testing.T) {
	options := &ClientOptions{ClientOptions: mqtt.NewClientOptions()}
	WithQos(1)(options)
	assert.Equal(t, byte(1), options.WillQos)
}

func TestWithMaxReconnectInterval(t *testing.T) {
	options := &ClientOptions{ClientOptions: mqtt.NewClientOptions()}
	WithMaxReconnectInterval(10 * time.Second)(options)
	assert.Equal(t, 10*time.Second, options.MaxReconnectInterval)
}

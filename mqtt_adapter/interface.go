package mqtt_adapter

//go:generate mockgen -source=interface.go -destination=mock/mock_mqtt.go

import (
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// Message represents a message in the MQTT protocol.
type Message = mqtt.Message

// MessageCallback is a function type that represents a callback for handling MQTT messages.
// It takes a pointer to a Client and a Message as parameters.
type MessageCallback func(MQTTClientAdapter, Message)

type MQTTClientAdapter interface {
	GetMqttClient() mqtt.Client
	OnConnectOnce(cb OnConnectCallback)
	OnConnectLostOnce(cb OnConnectLostCallback)
	OnConnect(cb OnConnectCallback) int
	OffConnect(idx int)
	OnConnectLost(cb OnConnectLostCallback) int
	OffConnectLost(idx int)
	Connect() error
	EnsureConnected()
	ConnectAndWaitForSuccess()
	Disconnect()
	IsConnected() bool
	Subscribe(topic string, qos byte, onMsg MessageCallback) mqtt.Token
	SubscribeMultiple(filters map[string]byte, onMsg MessageCallback) mqtt.Token
	SubscribeWait(topic string, qos byte, onMsg MessageCallback) error
	UnsubscribeAll() error
	Unsubscribe(topic string) error
	PublishBytes(topic string, qos byte, retained bool, data []byte) mqtt.Token
	publishObject(topic string, qos byte, payload interface{}) (mqtt.Token, error)
	Publish(topic string, qos byte, payload interface{}) error
	PublishWait(topic string, qos byte, payload interface{}) error
	PublishWaitTimeout(topic string, qos byte, timeout time.Duration, payload interface{}) error
}

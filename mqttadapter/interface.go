package mqttadapter

//go:generate mockgen -source=interface.go -destination=mock/mock_mqttadapter.go
//go:generate mockgen -package mock_mqtt -destination=mock/mqtt/mock_mqtt_client.go github.com/eclipse/paho.mqtt.golang Client,Token

import (
	"context"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// Message represents a message in the MQTT protocol.
type Message = mqtt.Message

// MessageCallback is a function type that represents a callback for handling MQTT messages.
// It takes a pointer to a Client and a Message as parameters.
type MessageCallback func(MQTTClientAdapter, Message)

// OnConnectCallback represents a callback function that is called when a connection is established.
type OnConnectCallback func()

// OnConnectLostCallback is a function type that represents a callback function
// to be called when the connection to the MQTT broker is lost.
// The callback function takes an error parameter that indicates the reason for
// the connection loss.
type OnConnectLostCallback func(err error)

// MQTTClientAdapter is an interface that defines the methods for interacting with an MQTT client.
type MQTTClientAdapter interface {
	// GetMqttClient returns the underlying MQTT client.
	GetMqttClient() mqtt.Client

	GetClientOptions() *mqtt.ClientOptions

	// OnConnectOnce sets a callback function to be called once when the client is connected.
	OnConnectOnce(cb OnConnectCallback)

	// OnConnectLostOnce sets a callback function to be called once when the client connection is lost.
	OnConnectLostOnce(cb OnConnectLostCallback)

	// OnConnect sets a callback function to be called when the client is connected.
	// It returns an index that can be used to remove the callback using OffConnect.
	OnConnect(cb OnConnectCallback) int

	// OffConnect removes the callback function associated with the given index.
	OffConnect(idx int)

	// OnConnectLost sets a callback function to be called when the client connection is lost.
	// It returns an index that can be used to remove the callback using OffConnectLost.
	OnConnectLost(cb OnConnectLostCallback) int

	// OffConnectLost removes the callback function associated with the given index.
	OffConnectLost(idx int)

	// Connect establishes a connection to the MQTT broker.
	// It takes a context.Context as a parameter and returns an error if the connection fails.
	Connect(ctx context.Context) error

	// EnsureConnected ensures that the client is connected to the MQTT broker.
	EnsureConnected()

	// ConnectAndWaitForSuccess establishes a connection to the MQTT broker and waits for a successful connection.
	ConnectAndWaitForSuccess()

	// Disconnect disconnects the client from the MQTT broker.
	Disconnect()

	// IsConnected returns true if the client is currently connected to the MQTT broker, false otherwise.
	IsConnected() bool

	// Subscribe subscribes to a topic with the specified quality of service (QoS) level and message callback function.
	Subscribe(ctx context.Context, topic string, qos byte, onMsg MessageCallback)

	// SubscribeWait subscribes to a topic with the specified quality of service (QoS) level and message callback function,
	// and waits for the subscription to be successful.
	SubscribeWait(ctx context.Context, topic string, qos byte, onMsg MessageCallback) error

	// SubscribeMultiple subscribes to multiple topics with the specified quality of service (QoS) levels and message callback function.
	SubscribeMultiple(ctx context.Context, filters map[string]byte, onMsg MessageCallback)

	// SubscribeMultipleWait subscribes to multiple topics with the specified quality of service (QoS) levels and message callback function,
	// and waits for the subscriptions to be successful.
	SubscribeMultipleWait(ctx context.Context, filters map[string]byte, onMsg MessageCallback) error

	// Unsubscribe unsubscribes from a topic.
	Unsubscribe(ctx context.Context, topic string)

	// UnsubscribeWait unsubscribes from a topic and waits for the unsubscribe to be successful.
	UnsubscribeWait(ctx context.Context, topic string) error

	// UnsubscribeAll unsubscribes from all topics.
	UnsubscribeAll(ctx context.Context)

	// UnsubscribeAllWait unsubscribes from all topics and waits for the unsubscribes to be successful.
	UnsubscribeAllWait(ctx context.Context) error

	// PublishBytes publishes a byte array as a message to the specified topic with the specified quality of service (QoS) level and retained flag.
	PublishBytes(ctx context.Context, topic string, qos byte, retained bool, data []byte)

	// PublishBytesWait publishes a byte array as a message to the specified topic with the specified quality of service (QoS) level and retained flag,
	// and waits for the publish to be successful.
	PublishBytesWait(ctx context.Context, topic string, qos byte, retained bool, data []byte) error

	// PublishObject publishes an object as a message to the specified topic with the specified quality of service (QoS) level and retained flag.
	PublishObject(ctx context.Context, topic string, qos byte, retained bool, payload any) error

	// PublishObjectWait publishes an object as a message to the specified topic with the specified quality of service (QoS) level and retained flag,
	// and waits for the publish to be successful.
	PublishObjectWait(ctx context.Context, topic string, qos byte, retained bool, payload any) error
}

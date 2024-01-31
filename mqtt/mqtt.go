package mqtt

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	stdlog "log"
	"net/url"
	"os"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"go.uber.org/zap"
)

// Message represents a message in the MQTT protocol.
type Message = mqtt.Message

// MessageCallback is a function type that represents a callback for handling MQTT messages.
// It takes a pointer to a Client and a Message as parameters.
type MessageCallback func(*Client, Message)

// Client represents an MQTT client.
type Client struct {
	client mqtt.Client

	subscribeMap sync.Map

	onConnectCallbakCount     int
	onConnectCallbakMutex     sync.Mutex
	onConnectCallbaks         map[int]OnConnectCallback
	onConnectLostCallbakCount int
	onConnectLostCallbakMutex sync.Mutex
	onConnectLostCallbaks     map[int]OnConnectLostCallback

	stopRetryConnect bool
	printableURL     string

	log *zap.SugaredLogger
}

// OnConnectCallback represents a callback function that is called when a connection is established.
type OnConnectCallback func()

// OnConnectLostCallback is a function type that represents a callback function
// to be called when the connection to the MQTT broker is lost.
// The callback function takes an error parameter that indicates the reason for
// the connection loss.
type OnConnectLostCallback func(err error)

// NewTLSConfig creates a new TLS configuration with the provided PEM certificates.
// The PEM certificates are used to import trusted certificates from CAfile.pem.
// The function returns a *tls.Config with the desired TLS properties, including
// the RootCAs used to verify the server certificate, the ClientAuth setting,
// the ClientCAs used to validate the client certificate, the InsecureSkipVerify
// option to skip server certificate verification, and the Certificates sent by
// the client to the server.
func NewTLSConfig(pemCerts []byte) *tls.Config {
	// Import trusted certificates from CAfile.pem.
	// Alternatively, manually add CA certificates to default openssl CA bundle.
	certpool := x509.NewCertPool()

	certpool.AppendCertsFromPEM(pemCerts)

	// Create tls.Config with desired tls properties
	return &tls.Config{
		// RootCAs = certs used to verify server cert.
		RootCAs: certpool,
		// ClientAuth = whether to request cert from server.
		// Since the server is set up for SSL, this happens
		// anyways.
		ClientAuth: tls.NoClientCert,
		// ClientCAs = certs used to validate client cert.
		ClientCAs: nil,
		// InsecureSkipVerify = verify that cert contents
		// match server. IP matches what is in cert etc.
		InsecureSkipVerify: true,
		// Certificates = list of certs client sends to server.
		// Certificates: []tls.Certificate{cert},
	}
}

// NewClient creates a new MQTT client with the specified URI, client ID, and options.
// The URI should be in the format "scheme://host:port", where scheme can be "tcp" or "ssl".
// The client ID is a unique identifier for the client.
// The options parameter allows for additional configuration of the client.
// Returns a pointer to the created Client and an error if any.
func NewClient(uri, clientID string, options ...Option) (*Client, error) {
	server, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}

	log := zap.S().With("module", "mqtt")

	clonedServer := *server
	clonedServer.User = nil
	client := Client{
		log:                   log,
		printableURL:          clonedServer.String(),
		stopRetryConnect:      false,
		onConnectCallbaks:     make(map[int]OnConnectCallback),
		onConnectLostCallbaks: make(map[int]OnConnectLostCallback),
	}

	mqttClientOptions := mqtt.NewClientOptions().
		AddBroker(uri).
		SetClientID(clientID).
		SetKeepAlive(60 * time.Second).
		SetTLSConfig(&tls.Config{}).
		SetDefaultPublishHandler(func(c mqtt.Client, m mqtt.Message) {
			log.Infof("DefaultPublishHandler %s %s", m.Topic(), string(m.Payload()))
		}).
		SetAutoReconnect(true).
		SetOnConnectHandler(func(mqtt.Client) {
			log.Infof("Connected %s", client.printableURL)
			for _, cb := range client.onConnectCallbaks {
				go cb()
			}
		}).
		SetConnectionLostHandler(func(c mqtt.Client, err error) {
			log.Infof("Connection lost %s %v", client.printableURL, err)
			for _, cb := range client.onConnectLostCallbaks {
				go cb(err)
			}
		})

	clientOptions := &ClientOptions{
		ClientOptions: mqttClientOptions,
	}

	for _, o := range options {
		o(clientOptions)
	}

	client.client = mqtt.NewClient(clientOptions.ClientOptions)

	if clientOptions.enableStatus {
		client.OnConnect(func() {
			client.PublishBytes(clientOptions.onlineTopic, 1, true, []byte(clientOptions.onlinePayload))
		})
	}

	if clientOptions.enableDebug {
		mqtt.DEBUG = stdlog.New(os.Stderr, "DEBUG - ", stdlog.LstdFlags)
		mqtt.CRITICAL = stdlog.New(os.Stderr, "CRITICAL - ", stdlog.LstdFlags)
		mqtt.WARN = stdlog.New(os.Stderr, "WARN - ", stdlog.LstdFlags)
		mqtt.ERROR = stdlog.New(os.Stderr, "ERROR - ", stdlog.LstdFlags)
	}

	return &client, nil
}

// GetMqttClient returns the MQTT client associated with the Client instance.
func (s *Client) GetMqttClient() *mqtt.Client {
	return &s.client
}

// OnConnectOnce registers a callback function to be executed once the MQTT client is connected.
// If the client is already connected, the callback function is executed immediately.
// Otherwise, the callback function is executed when the client successfully connects.
// The callback function is unregistered after it is executed.
func (s *Client) OnConnectOnce(cb OnConnectCallback) {
	if s.client.IsConnected() {
		cb()
		return
	}
	var subID int
	subID = s.OnConnect(func() {
		s.OffConnect(subID)
		cb()
	})
}

// OnConnectLostOnce registers a callback function to be called when the MQTT client loses connection to the broker.
// The callback function will be called only once and then automatically unregistered.
// The provided callback function should accept an error parameter, which represents the reason for the connection loss.
func (s *Client) OnConnectLostOnce(cb OnConnectLostCallback) {
	var subID int
	subID = s.OnConnectLost(func(err error) {
		s.OffConnectLost(subID)
		cb(err)
	})
}

// OnConnect registers a callback function to be called when the MQTT client is connected.
// The callback function will be invoked immediately if the client is already connected.
// The function returns an index that can be used to unregister the callback using the UnregisterOnConnect method.
func (s *Client) OnConnect(cb OnConnectCallback) int {
	if s.client.IsConnected() {
		cb()
	}

	s.onConnectCallbakMutex.Lock()
	defer s.onConnectCallbakMutex.Unlock()

	idx := s.onConnectCallbakCount
	s.onConnectCallbakCount++
	s.onConnectCallbaks[idx] = cb
	return idx
}

// OffConnect removes the onConnect callback function associated with the given index.
func (s *Client) OffConnect(idx int) {
	s.onConnectCallbakMutex.Lock()
	defer s.onConnectCallbakMutex.Unlock()

	delete(s.onConnectCallbaks, idx)
}

// OnConnectLost registers a callback function to be called when the MQTT client loses connection.
// The callback function will be invoked with an integer parameter representing the index of the callback.
// Returns the index of the registered callback.
func (s *Client) OnConnectLost(cb OnConnectLostCallback) int {
	s.onConnectLostCallbakMutex.Lock()
	defer s.onConnectLostCallbakMutex.Unlock()

	idx := s.onConnectLostCallbakCount
	s.onConnectLostCallbakCount++
	s.onConnectLostCallbaks[idx] = cb
	return idx
}

// OffConnectLost removes the callback function associated with the given index from the onConnectLostCallbaks map.
// It locks the onConnectLostCallbakMutex to ensure thread safety and then deletes the callback function from the map.
func (s *Client) OffConnectLost(idx int) {
	s.onConnectLostCallbakMutex.Lock()
	defer s.onConnectLostCallbakMutex.Unlock()

	delete(s.onConnectLostCallbaks, idx)
}

// Connect establishes a connection to the MQTT broker.
// It returns an error if the connection fails.
func (s *Client) Connect() error {
	if token := s.client.Connect(); token.Wait() && token.Error() != nil {
		return token.Error()
	}
	return nil
}

// EnsureConnected ensures that the MQTT client is connected.
// It starts a goroutine to connect to the MQTT broker and waits for the connection to be successful.
func (s *Client) EnsureConnected() {
	go s.ConnectAndWaitForSuccess()
}

// ConnectAndWaitForSuccess connects to the MQTT broker and waits for a successful connection.
// If the client is already connected, it returns immediately.
// If the connection fails, it retries every 10 seconds until a successful connection is established.
// The function stops retrying if the stopRetryConnect flag is set to true.
func (s *Client) ConnectAndWaitForSuccess() {
	if !s.IsConnected() {
		for !s.stopRetryConnect {
			if s.IsConnected() {
				s.log.Infof("mqtt is connected %s", s.printableURL)
				return
			}
			err := s.Connect()
			if err != nil {
				s.log.Errorf("Connect failed %s %v", s.printableURL, err)
				time.Sleep(time.Second * 10)
				s.log.Infof("Try reconnect %s", s.printableURL)
				continue
			}
			return
		}
		s.log.Infof("Stop retry connect %s", s.printableURL)
	}
}

// Disconnect disconnects the MQTT client from the broker.
// It stops the retry connection mechanism and calls the Disconnect method of the underlying MQTT client.
// The timeout parameter specifies the maximum time to wait for the disconnection to complete, in milliseconds.
func (s *Client) Disconnect() {
	s.stopRetryConnect = true
	s.client.Disconnect(1000)
}

// IsConnected returns a boolean value indicating whether the client is currently connected to the MQTT broker.
// It checks if the connection to the broker is open.
func (s *Client) IsConnected() bool {
	return s.client.IsConnectionOpen()
}

// Subscribe subscribes to a topic with the specified quality of service (QoS) level
// and registers a callback function to handle incoming messages.
// The topic parameter specifies the topic to subscribe to.
// The qos parameter specifies the desired QoS level for the subscription.
// The onMsg parameter is a callback function that will be called when a message is received.
// The callback function should have the following signature: func(client *Client, message mqtt.Message).
// The function returns a mqtt.Token that can be used to track the status of the subscription.
// If an error occurs during the subscription, it will be logged and returned as part of the token.
func (s *Client) Subscribe(topic string, qos byte, onMsg MessageCallback) mqtt.Token {
	s.log.Debugf("Subscribe topic=%s qos=%d", topic, qos)
	callback := func(c mqtt.Client, m mqtt.Message) {
		onMsg(s, m)
	}

	token := s.client.Subscribe(topic, qos, callback)
	if err := token.Error(); err != nil {
		s.log.With("f", "Subscribe").Errorf("%v", err)
	}

	s.subscribeMap.Store(topic, true)

	return token
}

// SubscribeMultiple subscribes to multiple MQTT topics with their respective QoS levels.
// It takes a map of topic filters and their corresponding QoS levels, and a callback function
// to handle incoming messages. The callback function is invoked for each message received
// on any of the subscribed topics.
//
// The function returns an mqtt.Token that can be used to track the status of the subscription.
// If there is an error during the subscription process, the error can be obtained from the token.
//
// The topics and their corresponding QoS levels are stored in the filters map. The onMsg
// callback function is invoked with the MQTT client and the received message for each
// message received on any of the subscribed topics.
//
// Example usage:
//
//	filters := map[string]byte{
//	  "topic1": 0,
//	  "topic2": 1,
//	  "topic3": 2,
//	}
//
//	onMsg := func(client *Client, message mqtt.Message) {
//	  // Handle incoming message
//	}
//
//	token := client.SubscribeMultiple(filters, onMsg)
//	if err := token.Error(); err != nil {
//	  // Handle subscription error
//	}
func (s *Client) SubscribeMultiple(filters map[string]byte, onMsg MessageCallback) mqtt.Token {
	s.log.Debugf("SubscribeMultiple topic=%v", filters)
	callback := func(c mqtt.Client, m mqtt.Message) {
		onMsg(s, m)
	}

	token := s.client.SubscribeMultiple(filters, callback)
	if err := token.Error(); err != nil {
		s.log.With("f", "Subscribe").Errorf("%v", err)
	}

	for topic := range filters {
		s.subscribeMap.Store(topic, true)
	}

	return token
}

// SubscribeWait subscribes to a topic with the specified QoS level and waits for the subscription to complete.
// It also registers a callback function to handle incoming messages on the subscribed topic.
// If there is an error during the subscription process, it returns the error.
func (s *Client) SubscribeWait(topic string, qos byte, onMsg MessageCallback) error {
	if token := s.Subscribe(topic, qos, onMsg); token.Wait() && token.Error() != nil {
		return token.Error()
	}

	return nil
}

// UnsubscribeAll unsubscribes from all topics that the client is currently subscribed to.
// It retrieves the list of topics from the subscribeMap and unsubscribes from each topic.
// After unsubscribing, it removes the topics from the subscribeMap.
// If any error occurs during the unsubscribe process, it returns the error.
// Otherwise, it returns nil.
func (s *Client) UnsubscribeAll() error {
	topics := []string{}
	s.subscribeMap.Range(func(key interface{}, value interface{}) bool {
		topics = append(topics, key.(string))
		return true
	})

	if token := s.client.Unsubscribe(topics...); token.Wait() && token.Error() != nil {
		return token.Error()
	}
	for _, topic := range topics {
		s.subscribeMap.Delete(topic)
	}
	return nil
}

// Unsubscribe unsubscribes from a topic.
// It removes the topic from the subscribeMap and sends an unsubscribe request to the MQTT broker.
// If there is an error during the unsubscribe process, it returns the error; otherwise, it returns nil.
func (s *Client) Unsubscribe(topic string) error {
	s.log.Debugf("Unsubscribe topic=%s", topic)
	s.subscribeMap.Delete(topic)
	if token := s.client.Unsubscribe(topic); token.Wait() && token.Error() != nil {
		return token.Error()
	}

	return nil
}

// PublishBytes publishes a message with the given topic, quality of service (qos), retained flag, and data.
// It returns an mqtt.Token representing the publish operation.
func (s *Client) PublishBytes(topic string, qos byte, retained bool, data []byte) mqtt.Token {
	return s.client.Publish(topic, qos, retained, data)
}

func (s *Client) publishObject(topic string, qos byte, payload interface{}) (mqtt.Token, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	return s.client.Publish(topic, qos, false, data), nil
}

// Publish publishes a message to the specified topic with the given quality of service (QoS) level and payload.
// It returns an error if the message fails to be published.
func (s *Client) Publish(topic string, qos byte, payload interface{}) error {
	token, err := s.publishObject(topic, qos, payload)
	if err != nil {
		return err
	}
	return token.Error()
}

// PublishWait publishes a message to the specified topic with the given quality of service (qos) and payload.
// It waits for the message to be published and returns an error if there was a problem.
func (s *Client) PublishWait(topic string, qos byte, payload interface{}) error {
	token, err := s.publishObject(topic, qos, payload)
	if err != nil {
		return err
	}

	s.log.Debugf("Publish topic=%s payload=%v", topic, payload)
	if token.Wait() && token.Error() != nil {
		return token.Error()
	}

	return nil
}

// PublishWaitTimeout publishes a message to the MQTT broker with the specified topic, quality of service (QoS),
// timeout duration, and payload. It waits for the completion of the publish operation with the given timeout.
// If the operation times out or encounters an error, it returns the corresponding error.
func (s *Client) PublishWaitTimeout(topic string, qos byte, timeout time.Duration, payload interface{}) error {
	token, err := s.publishObject(topic, qos, payload)
	if err != nil {
		return err
	}

	s.log.Debugf("Publish topic=%s payload=%v", topic, payload)
	if token.WaitTimeout(timeout) && token.Error() != nil {
		return token.Error()
	}

	return nil
}

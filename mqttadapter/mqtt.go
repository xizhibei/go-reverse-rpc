package mqttadapter

import (
	"context"
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

// MQTTClientAdapterImpl represents an MQTT client.
type MQTTClientAdapterImpl struct {
	client        mqtt.Client
	clientOptions *ClientOptions

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

// New creates a new MQTT client with the specified URI, client ID, and options.
// The URI should be in the format "scheme://host:port", where scheme can be "tcp" or "ssl".
// The client ID is a unique identifier for the client.
// The options parameter allows for additional configuration of the client.
// Returns a pointer to the created Client and an error if any.
func New(uri, clientID string, options ...Option) (MQTTClientAdapter, error) {
	server, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}

	log := zap.S().With("module", "mqtt")

	clonedServer := *server
	clonedServer.User = nil
	client := MQTTClientAdapterImpl{
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
			client.PublishBytes(
				context.Background(),
				clientOptions.onlineTopic,
				1,
				true,
				[]byte(clientOptions.onlinePayload),
			)
		})
	}

	if clientOptions.enableDebug {
		mqtt.DEBUG = stdlog.New(os.Stderr, "DEBUG - ", stdlog.LstdFlags)
		mqtt.CRITICAL = stdlog.New(os.Stderr, "CRITICAL - ", stdlog.LstdFlags)
		mqtt.WARN = stdlog.New(os.Stderr, "WARN - ", stdlog.LstdFlags)
		mqtt.ERROR = stdlog.New(os.Stderr, "ERROR - ", stdlog.LstdFlags)
	}

	client.clientOptions = clientOptions

	return &client, nil
}

// GetMqttClient returns the MQTT client associated with the Client instance.
func (s *MQTTClientAdapterImpl) GetMqttClient() mqtt.Client {
	return s.client
}

func (s *MQTTClientAdapterImpl) GetClientOptions() *mqtt.ClientOptions {
	return s.clientOptions.ClientOptions
}

// OnConnectOnce registers a callback function to be executed once the MQTT client is connected.
// If the client is already connected, the callback function is executed immediately.
// Otherwise, the callback function is executed when the client successfully connects.
// The callback function is unregistered after it is executed.
func (s *MQTTClientAdapterImpl) OnConnectOnce(cb OnConnectCallback) {
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
func (s *MQTTClientAdapterImpl) OnConnectLostOnce(cb OnConnectLostCallback) {
	var subID int
	subID = s.OnConnectLost(func(err error) {
		s.OffConnectLost(subID)
		cb(err)
	})
}

// OnConnect registers a callback function to be called when the MQTT client is connected.
// The callback function will be invoked immediately if the client is already connected.
// The function returns an index that can be used to unregister the callback using the UnregisterOnConnect method.
func (s *MQTTClientAdapterImpl) OnConnect(cb OnConnectCallback) int {
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
func (s *MQTTClientAdapterImpl) OffConnect(idx int) {
	s.onConnectCallbakMutex.Lock()
	defer s.onConnectCallbakMutex.Unlock()

	delete(s.onConnectCallbaks, idx)
}

// OnConnectLost registers a callback function to be called when the MQTT client loses connection.
// The callback function will be invoked with an integer parameter representing the index of the callback.
// Returns the index of the registered callback.
func (s *MQTTClientAdapterImpl) OnConnectLost(cb OnConnectLostCallback) int {
	s.onConnectLostCallbakMutex.Lock()
	defer s.onConnectLostCallbakMutex.Unlock()

	idx := s.onConnectLostCallbakCount
	s.onConnectLostCallbakCount++
	s.onConnectLostCallbaks[idx] = cb
	return idx
}

// OffConnectLost removes the callback function associated with the given index from the onConnectLostCallbaks map.
// It locks the onConnectLostCallbakMutex to ensure thread safety and then deletes the callback function from the map.
func (s *MQTTClientAdapterImpl) OffConnectLost(idx int) {
	s.onConnectLostCallbakMutex.Lock()
	defer s.onConnectLostCallbakMutex.Unlock()

	delete(s.onConnectLostCallbaks, idx)
}

func (s *MQTTClientAdapterImpl) Connect(ctx context.Context) error {
	token := s.client.Connect()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-token.Done():
		return token.Error()
	}
}

// EnsureConnected ensures that the MQTT client is connected.
// It starts a goroutine to connect to the MQTT broker and waits for the connection to be successful.
func (s *MQTTClientAdapterImpl) EnsureConnected() {
	go s.ConnectAndWaitForSuccess()
}

// ConnectAndWaitForSuccess connects to the MQTT broker and waits for a successful connection.
// If the client is already connected, it returns immediately.
// If the connection fails, it retries every 10 seconds until a successful connection is established.
// The function stops retrying if the stopRetryConnect flag is set to true.
func (s *MQTTClientAdapterImpl) ConnectAndWaitForSuccess() {
	ctx := context.Background()
	if !s.IsConnected() {
		for !s.stopRetryConnect {
			if s.IsConnected() {
				s.log.Infof("mqtt is connected %s", s.printableURL)
				return
			}
			err := s.Connect(ctx)
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
func (s *MQTTClientAdapterImpl) Disconnect() {
	s.stopRetryConnect = true
	s.client.Disconnect(1000)
}

// IsConnected returns a boolean value indicating whether the client is currently connected to the MQTT broker.
// It checks if the connection to the broker is open.
func (s *MQTTClientAdapterImpl) IsConnected() bool {
	return s.client.IsConnectionOpen()
}

// Subscribe subscribes to a topic with the specified quality of service (QoS) level
// and registers a callback function to handle incoming messages.
// The topic parameter specifies the topic to subscribe to.
// The qos parameter specifies the desired QoS level for the subscription.
// The onMsg parameter is a callback function that will be called when a message is received.
// The callback function receives the MQTT client adapter instance and the received message as parameters.
// The subscription is stored in the subscribeMap for later reference.
func (s *MQTTClientAdapterImpl) Subscribe(ctx context.Context, topic string, qos byte, onMsg MessageCallback) {
	s.log.Debugf("Subscribe topic=%s qos=%d", topic, qos)
	callback := func(c mqtt.Client, m mqtt.Message) {
		onMsg(s, m)
	}

	defer s.subscribeMap.Store(topic, true)

	s.client.Subscribe(topic, qos, callback)
}

// SubscribeWait subscribes to a topic with the specified quality of service (QoS) level
// and waits for incoming messages. It registers a callback function to handle each received message.
// The function returns an error if the context is canceled or if there is an error while subscribing.
//
// Parameters:
// - ctx: The context.Context object for cancellation.
// - topic: The topic to subscribe to.
// - qos: The quality of service level for the subscription.
// - onMsg: The callback function to handle incoming messages.
//
// Returns:
// - error: An error if the context is canceled or if there is an error while subscribing.
func (s *MQTTClientAdapterImpl) SubscribeWait(ctx context.Context, topic string, qos byte, onMsg MessageCallback) error {
	s.log.Debugf("Subscribe topic=%s qos=%d", topic, qos)

	callback := func(c mqtt.Client, m mqtt.Message) {
		onMsg(s, m)
	}

	defer s.subscribeMap.Store(topic, true)

	token := s.client.Subscribe(topic, qos, callback)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-token.Done():
		return token.Error()
	}
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
func (s *MQTTClientAdapterImpl) SubscribeMultiple(ctx context.Context, filters map[string]byte, onMsg MessageCallback) {
	s.log.Debugf("SubscribeMultiple topic=%v", filters)
	callback := func(c mqtt.Client, m mqtt.Message) {
		onMsg(s, m)
	}

	s.client.SubscribeMultiple(filters, callback)

	for topic := range filters {
		s.subscribeMap.Store(topic, true)
	}
}

// SubscribeMultipleWait subscribes to multiple MQTT topics and waits for incoming messages.
// It takes a context.Context object for cancellation, a map of topic filters and their QoS levels,
// and a MessageCallback function to handle incoming messages.
// The MessageCallback function is called with the MQTT client adapter and the received message as parameters.
// This function returns an error if the subscription or message handling encounters an error,
// or if the context is canceled before the subscription is completed.
func (s *MQTTClientAdapterImpl) SubscribeMultipleWait(ctx context.Context, filters map[string]byte, onMsg MessageCallback) error {
	callback := func(c mqtt.Client, m mqtt.Message) {
		onMsg(s, m)
	}

	defer func() {
		for topic := range filters {
			s.subscribeMap.Store(topic, true)
		}
	}()

	token := s.client.SubscribeMultiple(filters, callback)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-token.Done():
		return token.Error()
	}
}

// UnsubscribeAll unsubscribes from all topics that the client is currently subscribed to.
// It retrieves the list of topics from the subscribeMap and unsubscribes from each topic.
// After unsubscribing, it removes the topics from the subscribeMap.
func (s *MQTTClientAdapterImpl) UnsubscribeAll(ctx context.Context) {
	topics := []string{}
	s.subscribeMap.Range(func(key interface{}, value interface{}) bool {
		topics = append(topics, key.(string))
		return true
	})

	defer func() {
		for _, topic := range topics {
			s.subscribeMap.Delete(topic)
		}
	}()

	s.client.Unsubscribe(topics...)
}

// UnsubscribeAllWait unsubscribes from all topics that have been previously subscribed to.
// It waits for the operation to complete or for the context to be canceled.
// If the context is canceled before the operation completes, it returns the context error.
// If the operation completes with an error, it returns the error.
func (s *MQTTClientAdapterImpl) UnsubscribeAllWait(ctx context.Context) error {
	topics := []string{}
	s.subscribeMap.Range(func(key interface{}, value interface{}) bool {
		topics = append(topics, key.(string))
		return true
	})

	defer func() {
		for _, topic := range topics {
			s.subscribeMap.Delete(topic)
		}
	}()

	token := s.client.Unsubscribe(topics...)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-token.Done():
		return token.Error()
	}
}

// Unsubscribe unsubscribes from the specified MQTT topic.
// It takes a context.Context and the topic string as parameters.
// This method logs the topic being unsubscribed and removes it from the subscribeMap.
// Finally, it calls the Unsubscribe method of the MQTT client to unsubscribe from the topic.
func (s *MQTTClientAdapterImpl) Unsubscribe(ctx context.Context, topic string) {
	s.log.Debugf("Unsubscribe topic=%s", topic)

	defer s.subscribeMap.Delete(topic)

	s.client.Unsubscribe(topic)
}

// UnsubscribeWait unsubscribes from a topic and waits for the operation to complete or the context to be canceled.
// It returns an error if the operation fails or the context is canceled.
func (s *MQTTClientAdapterImpl) UnsubscribeWait(ctx context.Context, topic string) error {
	defer s.subscribeMap.Delete(topic)

	token := s.client.Unsubscribe(topic)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-token.Done():
		return token.Error()
	}
}

// PublishBytes publishes the given data as a byte array to the specified MQTT topic.
// It takes the context, topic, quality of service (QoS), retained flag, and data as parameters.
// The QoS determines the level of assurance for message delivery.
// The retained flag indicates whether the message should be retained by the broker.
// This function is used to publish data using the MQTT client.
func (s *MQTTClientAdapterImpl) PublishBytes(ctx context.Context, topic string, qos byte, retained bool, data []byte) {
	s.client.Publish(topic, qos, retained, data)
}

// PublishBytesWait publishes the given data as bytes to the specified topic with the specified quality of service (QoS),
// and waits for the operation to complete or the context to be canceled.
// It returns an error if the operation fails or if the context is canceled.
func (s *MQTTClientAdapterImpl) PublishBytesWait(ctx context.Context, topic string, qos byte, retained bool, data []byte) error {
	token := s.client.Publish(topic, qos, retained, data)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-token.Done():
		return token.Error()
	}
}

func (s *MQTTClientAdapterImpl) publishObject(ctx context.Context, topic string, qos byte, payload any) (mqtt.Token, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	return s.client.Publish(topic, qos, false, data), nil
}

// PublishObject publishes an object to the specified MQTT topic with the given quality of service (QoS),
// retained flag, and payload. It returns an error if the publishing operation fails.
func (s *MQTTClientAdapterImpl) PublishObject(ctx context.Context, topic string, qos byte, retained bool, payload any) error {
	_, err := s.publishObject(ctx, topic, qos, payload)
	if err != nil {
		return err
	}
	return nil
}

// PublishObjectWait publishes an object to the specified MQTT topic with the given quality of service (QoS),
// retention flag, and payload. It waits for the operation to complete or for the context to be canceled.
// If the context is canceled before the operation completes, it returns the context error.
// If the operation completes successfully, it returns nil.
// If there is an error during the operation, it returns the error from the MQTT token.
func (s *MQTTClientAdapterImpl) PublishObjectWait(ctx context.Context, topic string, qos byte, retained bool, payload any) error {
	token, err := s.publishObject(ctx, topic, qos, payload)
	if err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-token.Done():
		return token.Error()
	}
}

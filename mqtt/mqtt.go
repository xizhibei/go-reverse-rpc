package mqtt

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"net/url"
	"os"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"go.uber.org/zap"
)

// Message ...
type Message = mqtt.Message

type MessageCallback func(*Client, Message)

// Client ...
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

type OnConnectCallback func()
type OnConnectLostCallback func(err error)

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

// NewClient ...
func NewClient(uri, clientID string, options ...Option) (*Client, error) {
	server, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}

	log := zap.S().With("module", "iot")

	// mqtt.DEBUG = stdlog.New(os.Stderr, "DEBUG - ", stdlog.LstdFlags)
	// mqtt.CRITICAL = stdlog.New(os.Stderr, "CRITICAL - ", stdlog.LstdFlags)
	// mqtt.WARN = stdlog.New(os.Stderr, "WARN - ", stdlog.LstdFlags)
	// mqtt.ERROR = stdlog.New(os.Stderr, "ERROR - ", stdlog.LstdFlags)

	clonedServer := *server
	clonedServer.User = nil
	client := Client{
		log:                   log,
		printableURL:          clonedServer.String(),
		stopRetryConnect:      false,
		onConnectCallbaks:     make(map[int]OnConnectCallback),
		onConnectLostCallbaks: make(map[int]OnConnectLostCallback),
	}

	tmp := os.TempDir() + "/mqtt"
	_ = os.Mkdir(tmp, os.ModePerm)

	mqttClientOptions := mqtt.NewClientOptions().
		AddBroker(uri).
		SetClientID(clientID).
		SetKeepAlive(60 * time.Second).
		SetTLSConfig(&tls.Config{}).
		SetStore(mqtt.NewFileStore(tmp)).
		SetDefaultPublishHandler(func(c mqtt.Client, m mqtt.Message) {
			log.Infof("DefaultPublishHandler %s %s", m.Topic(), string(m.Payload()))
		}).
		SetAutoReconnect(true). //断网自动重连
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

	return &client, nil
}

func (s *Client) GetMqttClient() *mqtt.Client {
	return &s.client
}

// OnConnectOnce ...
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

// OnConnectLostOnce ...
func (s *Client) OnConnectLostOnce(cb OnConnectLostCallback) {
	var subID int
	subID = s.OnConnectLost(func(err error) {
		s.OffConnectLost(subID)
		cb(err)
	})
}

// OnConnect ...
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

// OffConnect ...
func (s *Client) OffConnect(idx int) {
	s.onConnectCallbakMutex.Lock()
	defer s.onConnectCallbakMutex.Unlock()

	delete(s.onConnectCallbaks, idx)
}

// OnConnectLost ...
func (s *Client) OnConnectLost(cb OnConnectLostCallback) int {
	s.onConnectLostCallbakMutex.Lock()
	defer s.onConnectLostCallbakMutex.Unlock()

	idx := s.onConnectLostCallbakCount
	s.onConnectLostCallbakCount++
	s.onConnectLostCallbaks[idx] = cb
	return idx
}

// OffConnectLost ...
func (s *Client) OffConnectLost(idx int) {
	s.onConnectLostCallbakMutex.Lock()
	defer s.onConnectLostCallbakMutex.Unlock()

	delete(s.onConnectLostCallbaks, idx)
}

// Connect ...
func (s *Client) Connect() error {
	if token := s.client.Connect(); token.Wait() && token.Error() != nil {
		return token.Error()
	}
	return nil
}

func (s *Client) ConnectForever() {
	go s.ConnectWaitSuccess()
}

// ConnectWaitSuccess 确保连接
func (s *Client) ConnectWaitSuccess() {
	if !s.IsConnected() {
		for !s.stopRetryConnect {
			if s.IsConnected() { // 已连接的情况下 ，不需要再次连接。
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

// Disconnect ...
func (s *Client) Disconnect() {
	s.stopRetryConnect = true
	s.client.Disconnect(1000)
}

// IsConnected ...
func (s *Client) IsConnected() bool {
	return s.client.IsConnectionOpen()
}

// Subscribe ...
func (s *Client) Subscribe(topic string, qos byte, onMsg MessageCallback) mqtt.Token {
	// s.log.Infof("Subscribe topic=%s qos=%d", topic, qos)
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

// Subscribe ...
func (s *Client) SubscribeMultiple(filters map[string]byte, onMsg MessageCallback) mqtt.Token {
	// s.log.Infof("SubscribeMultiple topic=%s qos=%d", topic, qos)
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

// Subscribe ...
func (s *Client) SubscribeWait(topic string, qos byte, onMsg MessageCallback) error {
	if token := s.Subscribe(topic, qos, onMsg); token.Wait() && token.Error() != nil {
		return token.Error()
	}

	return nil
}

// UnsubscribeAll ...
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

func (s *Client) Unsubscribe(topic string) error {
	// s.log.Infof("Unsubscribe topic=%s", topic)
	s.subscribeMap.Delete(topic)
	if token := s.client.Unsubscribe(topic); token.Wait() && token.Error() != nil {
		return token.Error()
	}

	return nil
}

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

// Publish ...
func (s *Client) Publish(topic string, qos byte, payload interface{}) error {
	token, err := s.publishObject(topic, qos, payload)
	if err != nil {
		return err
	}
	return token.Error()
}

func (s *Client) PublishWait(topic string, qos byte, payload interface{}) error {
	token, err := s.publishObject(topic, qos, payload)
	if err != nil {
		return err
	}

	// s.log.Infof("Publish topic=%s payload=%s", topic, string(data))
	if token.Wait() && token.Error() != nil {
		return token.Error()
	}

	return nil
}

func (s *Client) PublishWaitTimeout(topic string, qos byte, timeout time.Duration, payload interface{}) error {
	token, err := s.publishObject(topic, qos, payload)
	if err != nil {
		return err
	}

	// s.log.Infof("Publish topic=%s payload=%s", topic, string(data))
	if token.WaitTimeout(timeout) && token.Error() != nil {
		return token.Error()
	}

	return nil
}

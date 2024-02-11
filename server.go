package reverse_rpc

import (
	"errors"
	"fmt"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/Jeffail/tunny"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/time/rate"
)

var (
	// ErrNoReply is an error indicating an empty reply.
	ErrNoReply = errors.New("[RRPC] empty reply")

	// ErrTooFraquently is an error indicating that the request was made too frequently.
	ErrTooFraquently = errors.New("[RRPC] too frequently, try again later")

	// ErrTimeout is an error indicating a timeout occurred.
	ErrTimeout = errors.New("[RRPC] timeout")
)

// Server represents a reverse RPC server.
type Server struct {
	log        *zap.SugaredLogger  // Logger for server logs.
	handlerMap map[string]*Handler // Map of registered handlers.
	handlerMu  sync.Mutex          // Mutex to synchronize access to handlerMap.

	cbList       []OnAfterResponseCallback // List of callbacks to be executed after each response.
	afterResPool sync.Pool                 // Pool of resources for after-response processing.

	options    *serverOptions // Options for server configuration.
	workerPool *tunny.Pool    // Pool of worker goroutines for request processing.
	limiter    *rate.Limiter  // Rate limiter for controlling request rate.
}

type serverOptions struct {
	logResponse     bool
	name            string
	workerNum       int
	limiterDuration time.Duration
	limiterCount    int
}

// ServerOption is a functional option for configuring the server.
type ServerOption func(o *serverOptions)

// WithServerName is a function that returns a ServerOption to set the name of the server.
// The name parameter specifies the name of the server.
// It returns a function that takes a pointer to serverOptions and sets the name field.
func WithServerName(name string) ServerOption {
	return func(o *serverOptions) {
		o.name = name
	}
}

// WithLogResponse is a function that returns a ServerOption to enable or disable logging of response.
// It takes a boolean parameter logResponse, which determines whether to log the response or not.
// The returned ServerOption modifies the serverOptions struct by setting the logResponse field.
func WithLogResponse(logResponse bool) ServerOption {
	return func(o *serverOptions) {
		o.logResponse = logResponse
	}
}

// WithLimiter is a function that returns a ServerOption which sets the limiter duration and count for the server.
// The limiter duration specifies the time window in which the server can handle a certain number of requests.
// The limiter count specifies the maximum number of requests that can be handled within the specified time window.
func WithLimiter(d time.Duration, count int) ServerOption {
	return func(o *serverOptions) {
		o.limiterDuration = d
		o.limiterCount = count
	}
}

// WithWorkerNum is a function that returns a ServerOption which sets the number of workers for the server.
// The count parameter specifies the number of workers to be set.
func WithWorkerNum(count int) ServerOption {
	return func(o *serverOptions) {
		o.workerNum = count
	}
}

// NewServer creates a new instance of the Server struct with the provided options.
// It initializes the server with default values for the options that are not provided.
// The server instance is returned as a pointer.
func NewServer(options ...ServerOption) *Server {
	o := serverOptions{
		name:            uuid.New().String(),
		logResponse:     false,
		workerNum:       runtime.NumCPU(),
		limiterDuration: time.Second,
		limiterCount:    5,
	}

	for _, option := range options {
		option(&o)
	}

	rt := rate.Every(o.limiterDuration)
	limiter := rate.NewLimiter(rt, o.limiterCount)

	server := Server{
		log:        zap.S().With("module", "rrpc.server"),
		handlerMap: make(map[string]*Handler),
		options:    &o,

		afterResPool: sync.Pool{
			New: func() interface{} {
				return new(AfterResponseEvent)
			},
		},
		workerPool: tunny.NewCallback(o.workerNum),
		limiter:    limiter,
	}

	return &server
}

// Register registers a method with its corresponding handler in the server.
// If the method is already registered, it will be overridden.
// The method parameter specifies the name of the method.
// The hdl parameter is a pointer to the handler that will be associated with the method.
func (s *Server) Register(method string, hdl *Handler) {
	s.handlerMu.Lock()
	defer s.handlerMu.Unlock()

	if _, ok := s.handlerMap[method]; ok {
		s.log.Warn("Method %s already registered, will override", method)
	}

	s.handlerMap[method] = hdl
	s.log.Debugf("Method %s registered", method)
}

// Call handles the RPC call by executing the specified method and processing the response.
// It measures the duration of the call, logs the response if enabled, and emits an event after the response.
// If the call exceeds the timeout or encounters an error, it replies with an appropriate error message.
func (s *Server) Call(c Context) {
	start := time.Now()
	defer func() {
		duration := time.Since(start).Round(time.Millisecond)

		evt := s.afterResPool.Get().(*AfterResponseEvent)
		evt.Labels = c.PrometheusLabels()
		evt.Duration = duration
		evt.Res = c.GetResponse()

		if s.options.logResponse {
			status := 0
			if evt.Res != nil {
				status = evt.Res.Status
			}

			s.log.Infof("Response to %s [%d] (%v)", c.ReplyDesc(), status, duration)
		}

		s.emitAfterResponse(evt)
	}()

	err := s.limiter.Wait(c.Ctx())
	if err != nil {
		c.ReplyError(RPCStatusRequestTimeout, ErrTimeout)
		return
	}
	// if !s.limiter.Allow() {
	// 	c.ReplyError(429, ErrTooFraquently)
	// 	return
	// }

	hdl, ok := s.handlerMap[c.Method()]
	if !ok {
		c.ReplyError(RPCStatusServerError, fmt.Errorf("Unhandled method: %s", c.Method()))
		return
	}

	_, err = s.workerPool.ProcessTimed(func() {
		defer func() {
			if i := recover(); i != nil {
				err := fmt.Errorf("panic in method %s %v", c.Method(), i)
				s.log.Desugar().WithOptions(zap.AddStacktrace(zapcore.ErrorLevel)).Sugar().Error(err)
				c.ReplyError(RPCStatusServerError, err)
			}
		}()

		hdl.Method(c)

		// If the send is successful, it means that the method did not reply with any message.
		if c.ReplyError(RPCStatusServerError, ErrNoReply) {
			s.log.Warnf("Method %s no reply", c.Method())
		}

	}, hdl.Timeout)

	if err != nil {
		c.ReplyError(RPCStatusServerError, err)
	}
}

// Labels associated with the response event.
// Duration of the response event.
// The response object.
type AfterResponseEvent struct {
	Labels   prometheus.Labels
	Duration time.Duration
	Res      *Response
}

// OnAfterResponseCallback is a function type that represents a callback function
// to be executed after a response is sent. It takes a pointer to an AfterResponseEvent
// as its parameter.
type OnAfterResponseCallback func(e *AfterResponseEvent)

// OnAfterResponse registers a callback function to be executed after each response is sent.
// The provided callback function will be added to the callback list of the server.
// The callback function will be called with the response as its argument.
func (s *Server) OnAfterResponse(cb OnAfterResponseCallback) {
	s.cbList = append(s.cbList, cb)
}

func (s *Server) emitAfterResponse(e *AfterResponseEvent) {
	for _, cb := range s.cbList {
		cb(e)
	}
	s.afterResPool.Put(e)
}

// RegisterMetrics registers metrics for monitoring the server's response time and error count.
// It takes two parameters: responseTime, a Prometheus HistogramVec for tracking response time,
// and errorCount, a Prometheus GaugeVec for counting errors.
// The function adds an event listener to the server's OnAfterResponse event, which is triggered
// after each response is sent. Inside the event listener, it extracts the response status and
// labels from the event, and updates the labels with additional information. It then uses the
// responseTime HistogramVec to record the response time, and the errorCount GaugeVec to increment
// the error count if there is an error in the response.
func (s *Server) RegisterMetrics(responseTime *prometheus.HistogramVec, errorCount *prometheus.GaugeVec) {
	s.OnAfterResponse(func(e *AfterResponseEvent) {
		status := "0"
		if e.Res != nil {
			status = strconv.FormatInt(int64(e.Res.Status), 10)
		}

		labels := e.Labels
		labels["name"] = s.options.name
		labels["status"] = status

		if responseTime != nil {
			responseTime.
				With(labels).
				Observe(e.Duration.Seconds())
		}

		if e.Res.Error != nil && errorCount != nil {
			labels["message"] = e.Res.Error.Error()
			errorCount.
				With(labels).
				Inc()
		}
	})
}

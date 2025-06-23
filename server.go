package reverserpc

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
	"github.com/xizhibei/go-reverse-rpc/telemetry"
	"go.opentelemetry.io/otel/trace"
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

	options    *serverOptions      // Options for server configuration.
	workerPool *tunny.Pool         // Pool of worker goroutines for request processing.
	limiter    *rate.Limiter       // Rate limiter for controlling request rate.
	telemetry  telemetry.Telemetry // OpenTelemetry components
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
		limiterReject:   true,
	}

	for _, option := range options {
		option(&o)
	}

	rt := rate.Every(o.limiterDuration)
	limiter := rate.NewLimiter(rt, o.limiterCount)

	tel, _ := telemetry.NewNoop()

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
		telemetry:  tel,
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
	ctx := c.Ctx()

	var span trace.Span
	ctx, span = s.telemetry.StartSpan(ctx, "RRPC.Server.Call "+c.Method())
	defer span.End()

	start := time.Now()
	defer func() {
		duration := time.Since(start).Round(time.Millisecond)

		evt := s.afterResPool.Get().(*AfterResponseEvent)
		evt.Duration = duration
		evt.Res = c.GetResponse()

		if s.options.logResponse {
			status := 0
			if evt.Res != nil {
				status = evt.Res.Status
			}

			s.log.Infof("Response to %s [%d] (%v)", c.ReplyDesc(), status, duration)
		}

		status := "0"
		if evt.Res != nil {
			status = strconv.FormatInt(int64(evt.Res.Status), 10)
		}
		s.telemetry.RecordRequest(ctx, duration, c.Method(), status, evt.Res.Error)

		s.emitAfterResponse(evt)
	}()

	if s.options.limiterReject {
		if !s.limiter.Allow() {
			c.ReplyError(RPCStatusTooFraquently, ErrTooFraquently)
			return
		}
	} else {
		err := s.limiter.Wait(c.Ctx())
		if err != nil {
			c.ReplyError(RPCStatusRequestTimeout, ErrTimeout)
			return
		}
	}

	hdl, ok := s.handlerMap[c.Method()]
	if !ok {
		c.ReplyError(RPCStatusServerError, fmt.Errorf("Unhandled method: %s", c.Method()))
		return
	}

	_, err := s.workerPool.ProcessTimed(func() {
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

// AfterResponseEvent represents an event that is emitted after a response is sent.
type AfterResponseEvent struct {
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

// @Deprecated use OpenTelemetry instead
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

		labels := prometheus.Labels{}
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

// SetTelemetry sets the telemetry for the server
func (s *Server) SetTelemetry(tel telemetry.Telemetry) {
	s.telemetry = tel
}

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
	ErrNoReply       = errors.New("empty reply")
	ErrTooFraquently = errors.New("调用过于频繁，请稍后再试")
	ErrTimeout       = errors.New("请求超时")
)

type Server struct {
	log        *zap.SugaredLogger
	handlerMap map[string]*Handler
	handlerMu  sync.Mutex

	cbList       []OnAfterResponseCallback
	afterResPool sync.Pool

	options    *serverOptions
	workerPool *tunny.Pool
	limiter    *rate.Limiter
}

type serverOptions struct {
	logResponse     bool
	name            string
	workerNum       int
	limiterDuration time.Duration
	limiterCount    int
}

type ServerOption func(o *serverOptions)

func WithServerName(name string) ServerOption {
	return func(o *serverOptions) {
		o.name = name
	}
}

func WithLogResponse(logResponse bool) ServerOption {
	return func(o *serverOptions) {
		o.logResponse = logResponse
	}
}

func WithLimiter(d time.Duration, count int) ServerOption {
	return func(o *serverOptions) {
		o.limiterDuration = d
		o.limiterCount = count
	}
}

func WithWorkerNum(count int) ServerOption {
	return func(o *serverOptions) {
		o.workerNum = count
	}
}

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
		log:        zap.S().With("module", "rrpc"),
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

func (s *Server) Register(method string, hdl *Handler) {
	s.handlerMu.Lock()
	defer s.handlerMu.Unlock()

	if _, ok := s.handlerMap[method]; ok {
		s.log.Warn("Method %s already registered, will override", method)
	}

	s.handlerMap[method] = hdl
	// s.log.Infof("Method %s registered", method)
}

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

	err := s.limiter.Wait(c.Context())
	if err != nil {
		c.ReplyError(408, ErrTimeout)
		return
	}
	// if !s.limiter.Allow() {
	// 	c.ReplyError(429, ErrTooFraquently)
	// 	return
	// }

	hdl, ok := s.handlerMap[c.Method()]
	if !ok {
		c.ReplyError(500, fmt.Errorf("Unhandled method: %s", c.Method()))
		return
	}

	_, err = s.workerPool.ProcessTimed(func() {
		defer func() {
			if i := recover(); i != nil {
				err := fmt.Errorf("panic in method %s %v", c.Method(), i)
				s.log.Desugar().WithOptions(zap.AddStacktrace(zapcore.ErrorLevel)).Sugar().Error(err)
				c.ReplyError(500, err)
			}
		}()

		hdl.Method(c)

		// 这里如果发送成功，代表 Method 没有回复任何消息
		if c.ReplyError(500, ErrNoReply) {
			s.log.Warnf("Method %s no reply", c.Method())
		}

	}, hdl.Timeout)

	if err != nil {
		c.ReplyError(500, err)
	}
}

type AfterResponseEvent struct {
	Labels   prometheus.Labels
	Duration time.Duration
	Res      *Response
}

type OnAfterResponseCallback func(e *AfterResponseEvent)

func (s *Server) OnAfterResponse(cb OnAfterResponseCallback) {
	s.cbList = append(s.cbList, cb)
}

func (s *Server) emitAfterResponse(e *AfterResponseEvent) {
	for _, cb := range s.cbList {
		cb(e)
	}
	s.afterResPool.Put(e)
}

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

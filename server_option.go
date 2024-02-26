package reverserpc

import "time"

type serverOptions struct {
	logResponse     bool
	name            string
	workerNum       int
	limiterDuration time.Duration
	limiterCount    int
	limiterReject   bool
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
// Default values are 1 second and 5 requests.
func WithLimiter(d time.Duration, count int) ServerOption {
	return func(o *serverOptions) {
		o.limiterDuration = d
		o.limiterCount = count
	}
}

// WithLimiterReject returns a ServerOption that sets the limiterReject field of the serverOptions struct to true. This is default behavior.
// If you want the server to wait for available resources instead of rejecting requests when the limiter is full, use WithLimiterWait.
func WithLimiterReject() ServerOption {
	return func(o *serverOptions) {
		o.limiterReject = true
	}
}

// WithLimiterWait returns a ServerOption that sets the limiterReject field of the serverOptions struct to false.
// This allows the server to wait for available resources instead of rejecting requests when the limiter is full.
// If you want the server to reject requests when the limiter is full, use WithLimiterReject.
func WithLimiterWait() ServerOption {
	return func(o *serverOptions) {
		o.limiterReject = false
	}
}

// WithWorkerNum is a function that returns a ServerOption which sets the number of workers for the server.
// The count parameter specifies the number of workers to be set.
func WithWorkerNum(count int) ServerOption {
	return func(o *serverOptions) {
		o.workerNum = count
	}
}

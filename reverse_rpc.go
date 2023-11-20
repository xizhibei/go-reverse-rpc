package reverse_rpc

import "github.com/prometheus/client_golang/prometheus"

//go:generate mockgen -source=reverse_rpc.go -destination=mock/mock_reverse_rpc.go

type ReverseRPC interface {
	Close() error
	IsConnected() bool
	Register(method string, hdl *Handler)
	RegisterMetrics(responseTime *prometheus.HistogramVec, errorCount *prometheus.GaugeVec)
}

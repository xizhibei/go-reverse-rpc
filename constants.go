package reverserpc

const (
	RPCStatusOK             = 200
	RPCStatusClientError    = 400
	RPCStatusRequestTimeout = 408
	RPCStatusTooFraquently  = 429
	RPCStatusServerError    = 500

	DefaultQoS = 0
)

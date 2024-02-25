package mqttjson

import "encoding/json"

// Request represents a JSON-RPC request.
type Request struct {
	ID       uint64            `json:"id"`
	Method   string            `json:"method"`
	Metadata map[string]string `json:"metadata,omitempty"`
	Params   json.RawMessage   `json:"params"`
}

// Response represents a JSON-RPC response.
type Response struct {
	ID       uint64            `json:"id"`
	Method   string            `json:"method"`
	Status   int               `json:"status"`
	Metadata map[string]string `json:"metadata,omitempty"`
	Data     json.RawMessage   `json:"data"`
}

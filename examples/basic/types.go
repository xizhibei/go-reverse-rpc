package basic

// MathRequest represents a math operation request
type MathRequest struct {
	A float64 `json:"a"`
	B float64 `json:"b"`
}

// MathResponse represents a math operation response
type MathResponse struct {
	Result float64 `json:"result"`
}

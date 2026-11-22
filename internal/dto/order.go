package dto

import "encoding/json"

// Order is the public order response; accrual is an exact JSON number in points.
// Internal order and owner IDs are intentionally absent.
type Order struct {
	Number     string       `json:"number"`
	Status     string       `json:"status"`
	Accrual    *json.Number `json:"accrual,omitempty"`
	UploadedAt string       `json:"uploaded_at"`
}

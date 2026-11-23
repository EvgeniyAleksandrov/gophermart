package accrual

import "encoding/json"

type responseDTO struct {
	Order   string          `json:"order"`
	Status  string          `json:"status"`
	Accrual json.RawMessage `json:"accrual"`
}

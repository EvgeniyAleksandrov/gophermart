package dto

import "encoding/json"

type AccrualResponse struct {
	Order   string          `json:"order"`
	Status  string          `json:"status"`
	Accrual json.RawMessage `json:"accrual"`
}

package dto

import "encoding/json"

type Order struct {
	Number     string       `json:"number"`
	Status     string       `json:"status"`
	Accrual    *json.Number `json:"accrual,omitempty"`
	UploadedAt string       `json:"uploaded_at"`
}

package dto

import "encoding/json"

// Balance is the public balance response, including zero values.
type Balance struct {
	Current   json.Number `json:"current"`
	Withdrawn json.Number `json:"withdrawn"`
}

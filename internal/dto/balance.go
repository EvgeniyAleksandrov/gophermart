package dto

import "encoding/json"

type Balance struct {
	Current   json.Number `json:"current"`
	Withdrawn json.Number `json:"withdrawn"`
}

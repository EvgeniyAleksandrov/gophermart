package dto

import (
	"encoding/json"
	"errors"
	"io"
)

// ErrInvalidWithdrawalJSON indicates a malformed request or wrong required field type.
var ErrInvalidWithdrawalJSON = errors.New("invalid withdrawal JSON")

type WithdrawalRequest struct {
	Order string
	Sum   int64
}

func (r *WithdrawalRequest) Decode(reader io.Reader) error {
	*r = WithdrawalRequest{}
	var fields struct {
		Order *string         `json:"order"`
		Sum   json.RawMessage `json:"sum"`
	}
	decoder := json.NewDecoder(reader)
	if err := decoder.Decode(&fields); err != nil {
		return ErrInvalidWithdrawalJSON
	}

	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		return ErrInvalidWithdrawalJSON
	}

	if fields.Order == nil {
		return ErrInvalidWithdrawalJSON
	}

	sum, err := NewWithdrawalAmount().Parse(fields.Sum)
	if err != nil {
		return err
	}

	r.Order, r.Sum = *fields.Order, sum

	return nil
}

type Withdrawal struct {
	Order       string      `json:"order"`
	Sum         json.Number `json:"sum"`
	ProcessedAt string      `json:"processed_at"`
}

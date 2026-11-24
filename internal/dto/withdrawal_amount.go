package dto

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/EvgeniyAleksandrov/gophermart/internal/model"
)

type WithdrawalAmount struct {
}

func NewWithdrawalAmount() *WithdrawalAmount { return &WithdrawalAmount{} }

// Parse rejects nonnumbers as structure errors and invalid amounts without rounding.
// Decimal digits and a bounded scale avoid float64 and huge exponent allocations.
func (p *WithdrawalAmount) Parse(raw json.RawMessage) (int64, error) {
	raw = bytes.TrimSpace(raw)
	if !json.Valid(raw) || len(raw) == 0 || (raw[0] != '-' && (raw[0] < '0' || raw[0] > '9')) {
		return 0, ErrInvalidWithdrawalJSON
	}

	if raw[0] == '-' {
		return 0, model.ErrInvalidWithdrawalAmount
	}

	mantissa := string(raw)
	var exponent int64
	if i := strings.IndexAny(mantissa, "eE"); i >= 0 {
		var err error
		exponent, err = strconv.ParseInt(mantissa[i+1:], 10, 64)
		if err != nil || exponent < -int64(len(raw))-2 || exponent > int64(len(raw))+19 {
			return 0, model.ErrInvalidWithdrawalAmount
		}

		mantissa = mantissa[:i]
	}

	var fraction int64
	if i := strings.IndexByte(mantissa, '.'); i >= 0 {
		fraction = int64(len(mantissa) - i - 1)
		mantissa = mantissa[:i] + mantissa[i+1:]
	}

	digits := strings.TrimLeft(mantissa, "0")
	if digits == "" {
		return 0, model.ErrInvalidWithdrawalAmount
	}

	significant := strings.TrimRight(digits, "0")
	scale := exponent + 2 - fraction + int64(len(digits)-len(significant))
	if scale < 0 || scale > 19 || int64(len(significant))+scale > 19 {
		return 0, model.ErrInvalidWithdrawalAmount
	}

	amount, err := strconv.ParseInt(significant+strings.Repeat("0", int(scale)), 10, 64)
	if err != nil {
		return 0, model.ErrInvalidWithdrawalAmount
	}

	return amount, nil
}

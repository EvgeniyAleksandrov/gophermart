package accrual

import (
	"bytes"
	"encoding/json"
	"math/big"

	"github.com/EvgeniyAleksandrov/gophermart/internal/model"
)

type AmountParser struct {
}

func NewAmountParser() *AmountParser { return &AmountParser{} }

func (p *AmountParser) Parse(raw json.RawMessage) (int64, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return 0, nil
	}

	if !json.Valid(raw) || (raw[0] != '-' && (raw[0] < '0' || raw[0] > '9')) {
		return 0, model.ErrAccrualInvalidResponse
	}

	value, ok := new(big.Rat).SetString(string(raw))
	if !ok || value.Sign() < 0 {
		return 0, model.ErrAccrualInvalidResponse
	}

	value.Mul(value, big.NewRat(100, 1))
	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(value.Num(), value.Denom(), remainder)
	if remainder.Lsh(remainder, 1).Cmp(value.Denom()) >= 0 {
		quotient.Add(quotient, big.NewInt(1))
	}

	if !quotient.IsInt64() || quotient.Sign() < 0 {
		return 0, model.ErrAccrualInvalidResponse
	}

	return quotient.Int64(), nil
}

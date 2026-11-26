package mapper

import (
	"encoding/json"
	"errors"
	"fmt"
)

type Amount struct {
}

func NewAmount() *Amount { return &Amount{} }

func (m *Amount) Number(amount int64) (json.Number, error) {
	if amount < 0 {
		return "", errors.New("present amount: negative value")
	}

	return json.Number(fmt.Sprintf("%d.%02d", amount/100, amount%100)), nil
}

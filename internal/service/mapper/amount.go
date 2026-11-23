package mapper

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Amount formats hundredths as exact JSON numbers.
type Amount struct{}

// NewAmount constructs a stateless formatter.
func NewAmount() *Amount { return &Amount{} }

// Number rejects negative amounts and preserves every decimal digit.
func (m *Amount) Number(amount int64) (json.Number, error) {
	if amount < 0 {
		return "", errors.New("present amount: negative value")
	}

	return json.Number(fmt.Sprintf("%d.%02d", amount/100, amount%100)), nil
}

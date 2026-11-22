// Package mapper converts business models into transport data.
package mapper

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/EvgeniyAleksandrov/gophermart/internal/dto"
	"github.com/EvgeniyAleksandrov/gophermart/internal/model"
)

// Order converts business orders into public response data.
type Order struct{}

// NewOrder constructs a stateless order mapper.
func NewOrder() *Order {
	return &Order{}
}

// Present preserves order and formats timestamps in UTC RFC3339.
// Nil accrual is omitted; nonnegative hundredths become exact JSON numbers.
func (m *Order) Present(orders []model.Order) ([]dto.Order, error) {
	result := make([]dto.Order, 0, len(orders))
	for _, order := range orders {
		response := dto.Order{
			Number:     order.Number,
			Status:     string(order.Status),
			UploadedAt: order.UploadedAt.UTC().Format(time.RFC3339),
		}

		if order.Accrual != nil {
			amount := *order.Accrual
			if amount < 0 {
				return nil, errors.New("present order: negative accrual")
			}

			number := json.Number(fmt.Sprintf("%d.%02d", amount/100, amount%100))
			response.Accrual = &number
		}

		result = append(result, response)
	}

	return result, nil
}

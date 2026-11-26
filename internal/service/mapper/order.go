// Package mapper converts business models into transport data.
package mapper

import (
	"fmt"
	"time"

	"github.com/EvgeniyAleksandrov/gophermart/internal/dto"
	"github.com/EvgeniyAleksandrov/gophermart/internal/model"
)

type Order struct {
}

func NewOrder() *Order {
	return &Order{}
}

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
			number, err := NewAmount().Number(amount)
			if err != nil {
				return nil, fmt.Errorf("present order: %w", err)
			}

			response.Accrual = &number
		}

		result = append(result, response)
	}

	return result, nil
}

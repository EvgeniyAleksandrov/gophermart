package mapper

import (
	"fmt"
	"time"

	"github.com/EvgeniyAleksandrov/gophermart/internal/dto"
	"github.com/EvgeniyAleksandrov/gophermart/internal/model"
)

type Withdrawal struct {
	amount *Amount
}

func NewWithdrawal() *Withdrawal { return &Withdrawal{amount: NewAmount()} }

func (m *Withdrawal) Present(withdrawals []model.Withdrawal) ([]dto.Withdrawal, error) {
	result := make([]dto.Withdrawal, 0, len(withdrawals))

	for _, withdrawal := range withdrawals {
		sum, err := m.amount.Number(withdrawal.Sum)
		if err != nil {
			return nil, fmt.Errorf("present withdrawal: %w", err)
		}

		result = append(result, dto.Withdrawal{
			Order:       withdrawal.OrderNumber,
			Sum:         sum,
			ProcessedAt: withdrawal.ProcessedAt.UTC().Format(time.RFC3339),
		})
	}

	return result, nil
}

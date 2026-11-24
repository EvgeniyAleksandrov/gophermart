package mapper

import (
	"github.com/EvgeniyAleksandrov/gophermart/internal/dto"
	"github.com/EvgeniyAleksandrov/gophermart/internal/model"
)

type Balance struct {
	amount *Amount
}

func NewBalance() *Balance { return &Balance{amount: NewAmount()} }

func (m *Balance) Present(balance model.Balance) (dto.Balance, error) {
	current, err := m.amount.Number(balance.Current)
	if err != nil {
		return dto.Balance{}, err
	}

	withdrawn, err := m.amount.Number(balance.Withdrawn)
	if err != nil {
		return dto.Balance{}, err
	}

	return dto.Balance{Current: current, Withdrawn: withdrawn}, nil
}

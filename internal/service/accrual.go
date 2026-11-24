//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -source=accrual.go -destination=accrual_mock_test.go -package=service_test

package service

import (
	"context"

	"github.com/EvgeniyAleksandrov/gophermart/internal/interfaces"
	"github.com/EvgeniyAleksandrov/gophermart/internal/model"
)

type AccrualClient interface {
	Fetch(context.Context, string) (model.AccrualResult, error)
}

type Accrual struct {
	storage interfaces.DBStorage
	client  AccrualClient
}

func NewAccrual(storage interfaces.DBStorage, client AccrualClient) *Accrual {
	return &Accrual{storage: storage, client: client}
}

func (s *Accrual) Pending(ctx context.Context) ([]model.Order, error) {
	return s.storage.ListPending(ctx)
}

func (s *Accrual) Process(ctx context.Context, order model.Order) error {
	result, err := s.client.Fetch(ctx, order.Number)
	if err != nil {
		return err
	}

	if err := s.storage.ApplyAccrual(ctx, order.ID, order.UserID, result); err != nil {
		return &model.PersistenceError{Err: err}
	}

	return nil
}

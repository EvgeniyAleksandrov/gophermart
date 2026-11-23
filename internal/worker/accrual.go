// Package worker processes unfinished orders sequentially until shutdown.
package worker

import (
	"context"
	"errors"
	"time"

	"github.com/EvgeniyAleksandrov/gophermart/internal/interfaces"
	"github.com/EvgeniyAleksandrov/gophermart/internal/model"
)

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -source=accrual.go -destination=accrual_mock_test.go -package=worker_test

const pollInterval = time.Second

// Processor selects unfinished orders and processes one external calculation.
type Processor interface {
	Pending(context.Context) ([]model.Order, error)
	Process(context.Context, model.Order) error
}

// Accrual runs one sequential queue without a per-order goroutine.
type Accrual struct {
	processor Processor
	logger    interfaces.Logger
}

// NewAccrual receives the prepared processor and shared logger.
func NewAccrual(processor Processor, logger interfaces.Logger) *Accrual {
	return &Accrual{processor: processor, logger: logger}
}

// Run starts immediately and polls indefinitely. Cancellation is normal completion.
func (w *Accrual) Run(ctx context.Context) error {
	for ctx.Err() == nil {
		if err := w.pass(ctx); err != nil && ctx.Err() == nil {
			w.logger.Error("accrual database operation failed")
		}

		if ctx.Err() != nil {
			return nil
		}

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(pollInterval):
		}
	}

	return nil
}

func (w *Accrual) pass(ctx context.Context) error {
	orders, err := w.processor.Pending(ctx)
	if err != nil {
		return err
	}

	for _, order := range orders {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		err := w.processor.Process(ctx, order)
		if ctx.Err() != nil {
			return ctx.Err()
		}

		var persistence *model.PersistenceError
		if errors.As(err, &persistence) {
			return err
		}

		if err != nil {
			w.logger.Warn("accrual request failed; order retained")
		}
	}

	return nil
}

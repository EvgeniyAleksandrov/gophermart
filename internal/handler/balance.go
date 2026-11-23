package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/EvgeniyAleksandrov/gophermart/internal/interfaces"
	"github.com/EvgeniyAleksandrov/gophermart/internal/model"
	"github.com/EvgeniyAleksandrov/gophermart/internal/requestcontext"
	"github.com/EvgeniyAleksandrov/gophermart/internal/service/mapper"
)

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -source=balance.go -destination=balance_mock_test.go -package=handler_test

// BalanceService supplies a checked user balance.
type BalanceService interface {
	Get(context.Context, int64) (model.Balance, error)
}

// Balance handles GET /api/user/balance.
type Balance struct {
	service     BalanceService
	mapper      *mapper.Balance
	userContext *requestcontext.User
	logger      interfaces.Logger
}

// NewBalance receives the balance service and shared logger.
func NewBalance(service BalanceService, logger interfaces.Logger) *Balance {
	return &Balance{
		service:     service,
		mapper:      mapper.NewBalance(),
		userContext: requestcontext.NewUser(),
		logger:      logger,
	}
}

// ServeHTTP returns both balance amounts as JSON, including zero values.
// Presentation completes before headers are written; JWTs are never refreshed here.
func (h *Balance) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.userContext.ID(r.Context())
	if !ok || userID <= 0 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), httpOperationTimeout)
	defer cancel()

	balance, err := h.service.Get(ctx, userID)
	if errors.Is(err, model.ErrUserNotFound) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if err != nil {
		h.logger.Error("get balance failed: internal error")
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	response, err := h.mapper.Present(balance)
	if err != nil {
		h.logger.Error("present balance failed: internal error")
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	body, err := json.Marshal(response)
	if err != nil {
		h.logger.Error("encode balance failed: internal error")
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(body); err != nil {
		h.logger.Error("write balance response failed")
	}
}

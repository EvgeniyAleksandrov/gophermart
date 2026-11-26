package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/EvgeniyAleksandrov/gophermart/internal/dto"
	"github.com/EvgeniyAleksandrov/gophermart/internal/interfaces"
	"github.com/EvgeniyAleksandrov/gophermart/internal/model"
	"github.com/EvgeniyAleksandrov/gophermart/internal/requestcontext"
)

type Withdraw struct {
	service     WithdrawalService
	userContext *requestcontext.User
	logger      interfaces.Logger
}

func NewWithdraw(service WithdrawalService, logger interfaces.Logger) *Withdraw {
	return &Withdraw{service: service, userContext: requestcontext.NewUser(), logger: logger}
}

// ServeHTTP parses one bounded JSON request and uses one budget for the entire debit.
// Success, including an identical repeat, has an empty body and does not refresh JWT.
func (h *Withdraw) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.userContext.ID(r.Context())
	if !ok || userID <= 0 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	body := http.MaxBytesReader(w, r.Body, maxWithdrawalBodyBytes)
	defer body.Close()

	var request dto.WithdrawalRequest
	if err := request.Decode(body); err != nil {
		if errors.Is(err, model.ErrInvalidWithdrawalAmount) {
			http.Error(w, "invalid withdrawal amount", http.StatusUnprocessableEntity)
			return
		}

		http.Error(w, "invalid request", http.StatusBadRequest)

		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), httpOperationTimeout)
	defer cancel()

	err := h.service.Withdraw(ctx, userID, request.Order, request.Sum)

	switch {
	case errors.Is(err, model.ErrInvalidWithdrawalAmount):
		http.Error(w, "invalid withdrawal amount", http.StatusUnprocessableEntity)
	case errors.Is(err, model.ErrInvalidOrderNumber):
		http.Error(w, "invalid order number", http.StatusUnprocessableEntity)
	case errors.Is(err, model.ErrUserNotFound):
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	case errors.Is(err, model.ErrInsufficientFunds):
		http.Error(w, "insufficient funds", http.StatusPaymentRequired)
	case errors.Is(err, model.ErrWithdrawalConflict):
		http.Error(w, "withdrawal conflict", http.StatusConflict)
	case err != nil:
		h.logger.Error("withdraw failed: internal error")
		http.Error(w, "internal server error", http.StatusInternalServerError)
	default:
		w.WriteHeader(http.StatusOK)
	}
}

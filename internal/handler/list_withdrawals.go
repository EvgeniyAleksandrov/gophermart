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

type ListWithdrawals struct {
	service     WithdrawalService
	mapper      *mapper.Withdrawal
	userContext *requestcontext.User
	logger      interfaces.Logger
}

func NewListWithdrawals(service WithdrawalService, logger interfaces.Logger) *ListWithdrawals {
	return &ListWithdrawals{
		service:     service,
		mapper:      mapper.NewWithdrawal(),
		userContext: requestcontext.NewUser(),
		logger:      logger,
	}
}

// ServeHTTP returns the user's withdrawals as JSON, or an empty 204 response.
// Presentation completes before headers are written; JWTs are never refreshed here.
func (h *ListWithdrawals) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.userContext.ID(r.Context())
	if !ok || userID <= 0 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), httpOperationTimeout)
	defer cancel()

	withdrawals, err := h.service.List(ctx, userID)
	if errors.Is(err, model.ErrUserNotFound) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if err != nil {
		h.logger.Error("list withdrawals failed: internal error")
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if len(withdrawals) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	response, err := h.mapper.Present(withdrawals)
	if err != nil {
		h.logger.Error("present withdrawals failed: internal error")
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	body, err := json.Marshal(response)
	if err != nil {
		h.logger.Error("encode withdrawals failed: internal error")
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(body); err != nil {
		h.logger.Error("write withdrawals response failed")
	}
}

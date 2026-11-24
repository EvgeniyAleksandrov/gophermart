package handler

import (
	"context"
	"errors"
	"io"
	"net/http"

	"github.com/EvgeniyAleksandrov/gophermart/internal/interfaces"
	"github.com/EvgeniyAleksandrov/gophermart/internal/model"
	"github.com/EvgeniyAleksandrov/gophermart/internal/requestcontext"
)

type UploadOrder struct {
	service     OrderService
	userContext *requestcontext.User
	logger      interfaces.Logger
}

func NewUploadOrder(service OrderService, logger interfaces.Logger) *UploadOrder {
	return &UploadOrder{
		service:     service,
		userContext: requestcontext.NewUser(),
		logger:      logger,
	}
}

// ServeHTTP reads a text number without a content-type or additional size limit.
// The owner comes exclusively from the authenticated request context.
func (h *UploadOrder) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	userID, ok := h.userContext.ID(r.Context())
	if !ok || userID <= 0 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), httpOperationTimeout)
	defer cancel()

	created, err := h.service.Upload(ctx, userID, string(body))

	switch {
	case errors.Is(err, model.ErrEmptyOrderNumber):
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	case errors.Is(err, model.ErrInvalidOrderNumber):
		http.Error(w, "invalid order number", http.StatusUnprocessableEntity)
		return
	case errors.Is(err, model.ErrUserNotFound):
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	case errors.Is(err, model.ErrOrderOwnedByAnotherUser):
		http.Error(w, "order already belongs to another user", http.StatusConflict)
		return
	case err != nil:
		h.logger.Error("upload order failed: internal error")
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if created {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	w.WriteHeader(http.StatusOK)
}

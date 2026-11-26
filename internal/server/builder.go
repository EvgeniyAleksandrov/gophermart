package server

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

type ServerBuilder struct {
	address string
	router  *chi.Mux
}

func NewServerBuilder(address string) *ServerBuilder {
	return &ServerBuilder{
		address: address,
		router:  chi.NewRouter(),
	}
}

func (b *ServerBuilder) AddHandler(method, path string, handler http.Handler) error {
	if err := b.validateMethod(method); err != nil {
		return fmt.Errorf("validate method: %w", err)
	}

	if err := b.validatePath(path); err != nil {
		return fmt.Errorf("validate path: %w", err)
	}

	if handler == nil {
		return fmt.Errorf("validate handler: %w", ErrHandlerNil)
	}

	b.router.Method(method, path, handler)

	return nil
}

func (b *ServerBuilder) validateMethod(method string) error {
	switch strings.ToUpper(method) {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut,
		http.MethodPatch, http.MethodDelete, http.MethodConnect, http.MethodOptions,
		http.MethodTrace, "QUERY":
		return nil
	default:
		return ErrMethodUnsupported
	}
}

func (b *ServerBuilder) validatePath(path string) error {
	if path == "" {
		return ErrPathEmpty
	}

	if !strings.HasPrefix(path, "/") {
		return ErrPathFormat
	}

	return nil
}

func (b *ServerBuilder) Build() (*http.Server, error) {
	return &http.Server{
		Addr:              b.address,
		Handler:           b.router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}, nil
}

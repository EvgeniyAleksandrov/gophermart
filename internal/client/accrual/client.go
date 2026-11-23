// Package accrual implements the external accrual HTTP protocol.
package accrual

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/EvgeniyAleksandrov/gophermart/internal/interfaces"
	"github.com/EvgeniyAleksandrov/gophermart/internal/model"
)

const accrualRateLimitFallback = 20 * time.Second

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -source=client.go -destination=client_mock_test.go -package=accrual_test

// HTTPDoer executes one request using a reusable transport.
type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// Client validates calculations and delegates complete request retries to Retryer.
type Client struct {
	baseURL string
	http    HTTPDoer
	parser  *AmountParser
	retryer interfaces.Retryer
}

// NewClient receives a validated base URL, shared HTTP client and retryer.
func NewClient(baseURL string, client HTTPDoer, retryer interfaces.Retryer) *Client {
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), http: client, parser: NewAmountParser(), retryer: retryer}
}

// Fetch retries complete requests according to the injected policy, validates
// the final response and never forwards user authorization.
func (c *Client) Fetch(ctx context.Context, number string) (model.AccrualResult, error) {
	var result model.AccrualResult
	err := c.retryer.Do(ctx, func() error {
		result = model.AccrualResult{}
		fetched, err := c.fetch(ctx, number)
		if err == nil {
			result = fetched
		}
		return err
	})
	if err != nil {
		return model.AccrualResult{}, err
	}
	return result, nil
}

func (c *Client) fetch(ctx context.Context, number string) (result model.AccrualResult, err error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/orders/"+url.PathEscape(number), nil)
	if err != nil {
		return model.AccrualResult{}, errors.Join(model.ErrAccrualUnavailable, err)
	}
	response, err := c.http.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return model.AccrualResult{}, ctx.Err()
		}

		return model.AccrualResult{}, errors.Join(model.ErrAccrualUnavailable, err)
	}
	defer func() {
		if ctx.Err() != nil {
			err = ctx.Err()
		}

		if closeErr := response.Body.Close(); closeErr != nil {
			err = errors.Join(err, model.ErrAccrualUnavailable, closeErr)
		}

		if err != nil {
			result = model.AccrualResult{}
		}
	}()
	switch response.StatusCode {
	case http.StatusNoContent:
		return model.AccrualResult{}, model.ErrAccrualNotRegistered
	case http.StatusTooManyRequests:
		return model.AccrualResult{}, &model.RateLimitError{RetryAfter: c.retryAfter(response.Header.Get("Retry-After"))}
	case http.StatusOK:
	default:
		return model.AccrualResult{}, model.ErrAccrualUnavailable
	}

	var dto *responseDTO
	decoder := json.NewDecoder(response.Body)
	if err := decoder.Decode(&dto); err != nil {
		if ctx.Err() != nil {
			return model.AccrualResult{}, ctx.Err()
		}

		var network net.Error
		if errors.As(err, &network) {
			return model.AccrualResult{}, errors.Join(model.ErrAccrualUnavailable, err)
		}

		return model.AccrualResult{}, model.ErrAccrualInvalidResponse
	}

	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		var network net.Error
		if errors.As(err, &network) {
			return model.AccrualResult{}, errors.Join(model.ErrAccrualUnavailable, err)
		}

		return model.AccrualResult{}, model.ErrAccrualInvalidResponse
	}

	if dto == nil || dto.Order != number {
		return model.AccrualResult{}, model.ErrAccrualInvalidResponse
	}
	amount, err := c.parser.Parse(dto.Accrual)
	if err != nil {
		return model.AccrualResult{}, err
	}

	status := model.AccrualStatus(dto.Status)
	switch status {
	case model.AccrualRegistered, model.AccrualProcessing, model.AccrualInvalid:
		amount = 0
	case model.AccrualProcessed:
	default:
		return model.AccrualResult{}, model.ErrAccrualInvalidResponse
	}

	return model.AccrualResult{Number: number, Status: status, Accrual: amount}, nil
}

func (c *Client) retryAfter(raw string) time.Duration {
	if raw == "" {
		return accrualRateLimitFallback
	}

	for _, digit := range raw {
		if digit < '0' || digit > '9' {
			return accrualRateLimitFallback
		}
	}
	seconds, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || seconds > uint64(math.MaxInt64/int64(time.Second)) {
		return accrualRateLimitFallback
	}

	return time.Duration(seconds) * time.Second
}

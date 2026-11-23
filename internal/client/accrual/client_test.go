package accrual_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/EvgeniyAleksandrov/gophermart/internal/client/accrual"
	"github.com/EvgeniyAleksandrov/gophermart/internal/model"
	"github.com/EvgeniyAleksandrov/gophermart/internal/service/retry"
	"github.com/EvgeniyAleksandrov/gophermart/internal/service/retry/strategy"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestClientFetch(t *testing.T) {
	for name, tc := range map[string]struct {
		code int
		body string
		want model.AccrualResult
		err  error
	}{
		"processed": {
			body: `{"order":"18","status":"PROCESSED","accrual":12.345,"future":true}`,
			want: model.AccrualResult{Number: "18", Status: model.AccrualProcessed, Accrual: 1235},
		},
		"missing amount": {
			body: `{"order":"18","status":"PROCESSED"}`,
			want: model.AccrualResult{Number: "18", Status: model.AccrualProcessed},
		},
		"null amount": {
			body: `{"order":"18","status":"PROCESSED","accrual":null}`,
			want: model.AccrualResult{Number: "18", Status: model.AccrualProcessed},
		},
		"registered": {
			body: `{"order":"18","status":"REGISTERED","accrual":12}`,
			want: model.AccrualResult{Number: "18", Status: model.AccrualRegistered},
		},
		"processing": {
			body: `{"order":"18","status":"PROCESSING","accrual":12}`,
			want: model.AccrualResult{Number: "18", Status: model.AccrualProcessing},
		},
		"invalid": {
			body: `{"order":"18","status":"INVALID","accrual":12}`,
			want: model.AccrualResult{Number: "18", Status: model.AccrualInvalid},
		},
		"invalid negative": {
			body: `{"order":"18","status":"INVALID","accrual":-0.001}`,
			err:  model.ErrAccrualInvalidResponse,
		},
		"processing overflow": {
			body: `{"order":"18","status":"PROCESSING","accrual":1e20}`,
			err:  model.ErrAccrualInvalidResponse,
		},
		"string amount": {
			body: `{"order":"18","status":"PROCESSED","accrual":"1"}`,
			err:  model.ErrAccrualInvalidResponse,
		},
		"wrong number": {
			body: `{"order":"19","status":"PROCESSED"}`,
			err:  model.ErrAccrualInvalidResponse,
		},
		"unknown status": {
			body: `{"order":"18","status":"NEW"}`,
			err:  model.ErrAccrualInvalidResponse,
		},
		"second value": {
			body: `{"order":"18","status":"PROCESSED"} {}`,
			err:  model.ErrAccrualInvalidResponse,
		},
		"trailing junk": {
			body: `{"order":"18","status":"PROCESSED"} x`,
			err:  model.ErrAccrualInvalidResponse,
		},
		"null object": {
			body: `null`,
			err:  model.ErrAccrualInvalidResponse,
		},
		"bad json": {
			body: `{`,
			err:  model.ErrAccrualInvalidResponse,
		},
		"not registered": {
			code: 204,
			err:  model.ErrAccrualNotRegistered,
		},
		"unavailable": {
			code: 500,
			err:  model.ErrAccrualUnavailable,
		},
		"unexpected code": {
			code: 403,
			err:  model.ErrAccrualUnavailable,
		},
	} {
		t.Run(name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				require.Equal(t, "GET", r.Method)
				require.Equal(t, "/api/orders/18", r.URL.Path)
				require.Empty(t, r.Header.Get("Authorization"))
				if tc.code != 0 {
					w.WriteHeader(tc.code)
				}
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			got, err := accrual.NewClient(server.URL+"/", server.Client(), oneAttempt(t)).Fetch(context.Background(), "18")
			require.Equal(t, 1, calls)
			if tc.err != nil {
				require.ErrorIs(t, err, tc.err)
				require.Zero(t, got)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestClientRateLimit(t *testing.T) {
	for name, tc := range map[string]struct {
		header string
		want   time.Duration
	}{
		"zero": {
			header: "0",
		},
		"twenty": {
			header: "20",
			want:   20 * time.Second,
		},
		"sixty": {
			header: "60",
			want:   time.Minute,
		},
		"missing": {
			want: 20 * time.Second,
		},
		"negative": {
			header: "-1",
			want:   20 * time.Second,
		},
		"date": {
			header: "Wed, 21 Oct 2015 07:28:00 GMT",
			want:   20 * time.Second,
		},
		"overflow": {
			header: "9223372037",
			want:   20 * time.Second,
		},
		"huge": {
			header: "9999999999999999999999999",
			want:   20 * time.Second,
		},
	} {
		t.Run(name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.Header().Set("Retry-After", tc.header)
				w.WriteHeader(429)
			}))
			defer server.Close()
			_, err := accrual.NewClient(server.URL, server.Client(), oneAttempt(t)).Fetch(context.Background(), "18")
			var limited *model.RateLimitError
			require.ErrorAs(t, err, &limited)
			require.Equal(t, tc.want, limited.RetryAfter)
			require.Equal(t, 1, calls)
		})
	}
}

func TestClientCancellationAndTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer server.Close()
	client := accrual.NewClient(server.URL, &http.Client{Timeout: 20 * time.Millisecond}, oneAttempt(t))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := client.Fetch(ctx, "18")
	require.ErrorIs(t, err, context.Canceled)
	_, err = client.Fetch(context.Background(), "18")
	require.ErrorIs(t, err, model.ErrAccrualUnavailable)
	require.True(t, errors.Is(err, context.DeadlineExceeded))
	server.Close()
	_, err = client.Fetch(context.Background(), "18")
	require.ErrorIs(t, err, model.ErrAccrualUnavailable)
}

type trackedBody struct {
	io.Reader
	closed   bool
	closeErr error
}

func (b *trackedBody) Close() error {
	b.closed = true
	return b.closeErr
}

func TestClientClosesBody(t *testing.T) {
	for name, tc := range map[string]struct {
		code     int
		body     string
		closeErr error
	}{
		"success": {
			code: 200,
			body: `{"order":"18","status":"PROCESSED"}`,
		},
		"invalid response": {
			code: 200,
			body: "{",
		},
		"not registered": {
			code: 204,
		},
		"limited": {
			code: 429,
		},
		"unavailable": {
			code: 500,
		},
		"close fails": {
			code:     200,
			body:     `{"order":"18","status":"PROCESSED"}`,
			closeErr: errors.New("close failed"),
		},
	} {
		t.Run(name, func(t *testing.T) {
			body := &trackedBody{Reader: strings.NewReader(tc.body), closeErr: tc.closeErr}
			doer := NewMockHTTPDoer(gomock.NewController(t))
			doer.EXPECT().Do(gomock.Any()).Return(&http.Response{StatusCode: tc.code, Body: body, Header: make(http.Header)}, nil)
			got, err := accrual.NewClient("http://accrual.test", doer, oneAttempt(t)).Fetch(context.Background(), "18")
			require.True(t, body.closed)
			if tc.closeErr != nil {
				require.ErrorIs(t, err, tc.closeErr)
				require.Zero(t, got)
			}
		})
	}
}

func oneAttempt(t *testing.T) *retry.Retryer {
	t.Helper()
	policy, err := strategy.NewExponential(1, 0, time.Second, strategy.NewSQLState(nil))
	require.NoError(t, err)
	return retry.NewRetryer(policy)
}

func TestClientRetriesRateLimitAndClosesEachAttempt(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		doer := NewMockHTTPDoer(gomock.NewController(t))
		first := &trackedBody{Reader: strings.NewReader("")}
		second := &trackedBody{Reader: strings.NewReader("")}
		final := &trackedBody{Reader: strings.NewReader(`{"order":"18","status":"PROCESSED","accrual":12.345}`)}
		started := time.Now()
		var previous *http.Request
		calls := 0
		doer.EXPECT().Do(gomock.Any()).Times(3).DoAndReturn(func(request *http.Request) (*http.Response, error) {
			require.NotSame(t, previous, request, "each attempt creates its own request")
			require.Equal(t, "/api/orders/18", request.URL.Path)
			require.Empty(t, request.Header.Get("Authorization"))
			previous = request
			calls++
			switch calls {
			case 1:
				return &http.Response{StatusCode: 429, Header: http.Header{"Retry-After": []string{"60"}}, Body: first}, nil
			case 2:
				require.True(t, first.closed)
				require.Equal(t, time.Minute, time.Since(started))
				return &http.Response{StatusCode: 429, Header: http.Header{"Retry-After": []string{"20"}}, Body: second}, nil
			default:
				require.True(t, second.closed)
				require.Equal(t, 80*time.Second, time.Since(started))
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: final}, nil
			}
		})
		client := accrual.NewClient("http://accrual.test", doer, retry.NewRetryer(strategy.NewRateLimit()))
		result, err := client.Fetch(context.Background(), "18")
		require.NoError(t, err)
		require.Equal(t, int64(1235), result.Accrual)
		require.True(t, final.closed)
	})
}

func TestClientRateLimitCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		doer := NewMockHTTPDoer(gomock.NewController(t))
		body := &trackedBody{Reader: strings.NewReader("")}
		doer.EXPECT().Do(gomock.Any()).Return(&http.Response{StatusCode: 429, Header: http.Header{"Retry-After": []string{"60"}}, Body: body}, nil)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		client := accrual.NewClient("http://accrual.test", doer, retry.NewRetryer(strategy.NewRateLimit()))
		done := make(chan error, 1)
		go func() {
			result, err := client.Fetch(ctx, "18")
			require.Zero(t, result)
			done <- err
		}()
		synctest.Wait()
		require.True(t, body.closed, "body must close before the retry wait")
		cancel()
		require.ErrorIs(t, <-done, context.Canceled)
	})
}

func TestClientStopsOnUnmatchedResponseAfterRateLimit(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(429)
			return
		}
		w.WriteHeader(500)
	}))
	defer server.Close()
	client := accrual.NewClient(server.URL, server.Client(), retry.NewRetryer(strategy.NewRateLimit()))
	result, err := client.Fetch(context.Background(), "18")
	require.ErrorIs(t, err, model.ErrAccrualUnavailable)
	require.Zero(t, result)
	require.Equal(t, 2, calls)
}

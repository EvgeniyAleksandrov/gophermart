package app_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/EvgeniyAleksandrov/gophermart/internal/app"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestAppCanceledAndListenFailure(t *testing.T) {
	for name, tc := range map[string]struct {
		canceled bool
		closeErr error
	}{
		"already canceled": {
			canceled: true,
		},
		"listen failure": {},
		"close failure": {
			canceled: true,
			closeErr: errors.New("close failed"),
		},
	} {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			storage, worker := NewMockStorage(ctrl), NewMockWorker(ctrl)
			workerExited := make(chan struct{})
			storage.EXPECT().Close().DoAndReturn(func() error {
				if !tc.canceled {
					select {
					case <-workerExited:
					default:
						t.Error("storage closed before worker exit")
					}
				}

				return tc.closeErr
			})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tc.canceled {
				cancel()
			} else {
				worker.EXPECT().Run(gomock.Any()).DoAndReturn(func(ctx context.Context) error { <-ctx.Done(); close(workerExited); return nil })
			}

			err := app.NewApp(&http.Server{Addr: "invalid address"}, zap.NewNop(), storage, worker).Run(ctx)
			switch {
			case tc.closeErr != nil:
				require.ErrorIs(t, err, tc.closeErr)
			case tc.canceled:
				require.NoError(t, err)
			default:
				require.ErrorContains(t, err, "listen HTTP")
			}
		})
	}
}

func TestAppDrainsWorkerAndHTTP(t *testing.T) {
	ctrl := gomock.NewController(t)
	storage, worker := NewMockStorage(ctrl), NewMockWorker(ctrl)
	workerExited := make(chan struct{})
	worker.EXPECT().Run(gomock.Any()).DoAndReturn(func(ctx context.Context) error {
		<-ctx.Done()
		close(workerExited)
		return nil
	})
	entered, canceled, release, exited := make(chan struct{}), make(chan struct{}), make(chan struct{}), make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-r.Context().Done()
		close(canceled)
		<-release
		close(exited)
	})
	storage.EXPECT().Close().DoAndReturn(func() error {
		select {
		case <-exited:
		default:
			t.Error("handler still active")
		}
		select {
		case <-workerExited:
		default:
			t.Error("worker still active")
		}

		return nil
	})
	core, logs := observer.New(zap.InfoLevel)
	server := &http.Server{Addr: "127.0.0.1:0", Handler: handler}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- app.NewApp(server, zap.New(core), storage, worker).Run(ctx) }()
	require.Eventually(t, func() bool { return logs.Len() > 0 }, time.Second, time.Millisecond)
	address := logs.All()[0].ContextMap()["address"].(string)
	requestDone := make(chan struct{})
	go func() {
		defer close(requestDone)
		client := &http.Client{Timeout: 2 * time.Second}
		response, err := client.Get("http://" + address)
		if err == nil {
			response.Body.Close()
		}
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("handler did not start")
	}
	cancel()
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("HTTP context not canceled")
	}
	select {
	case <-done:
		t.Fatal("storage closed while handler active")
	default:
	}
	close(release)
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("shutdown stuck")
	}
	<-requestDone
}

func TestAppWorkerFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	storage, worker := NewMockStorage(ctrl), NewMockWorker(ctrl)
	failure := errors.New("worker failed")
	worker.EXPECT().Run(gomock.Any()).Return(failure)
	storage.EXPECT().Close().Return(nil)
	err := app.NewApp(&http.Server{Addr: "127.0.0.1:0"}, zap.NewNop(), storage, worker).Run(context.Background())
	require.ErrorIs(t, err, failure)
}

func TestAppForcedCloseStillDrainsHandler(t *testing.T) {
	ctrl := gomock.NewController(t)
	storage, worker := NewMockStorage(ctrl), NewMockWorker(ctrl)
	worker.EXPECT().Run(gomock.Any()).DoAndReturn(func(ctx context.Context) error { <-ctx.Done(); return nil })
	entered, release, closed := make(chan struct{}), make(chan struct{}), make(chan struct{})
	handler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { close(entered); <-release })
	storage.EXPECT().Close().DoAndReturn(func() error { close(closed); return nil })
	core, logs := observer.New(zap.InfoLevel)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- app.NewApp(&http.Server{Addr: "127.0.0.1:0", Handler: handler}, zap.New(core), storage, worker).Run(ctx)
	}()
	require.Eventually(t, func() bool { return logs.Len() > 0 }, time.Second, time.Millisecond)
	address := logs.All()[0].ContextMap()["address"].(string)
	requestDone := make(chan error, 1)
	go func() {
		response, err := (&http.Client{Timeout: 15 * time.Second}).Get("http://" + address)
		if err == nil {
			response.Body.Close()
		}
		requestDone <- err
	}()
	<-entered
	cancel()
	select {
	case err := <-requestDone:
		require.Error(t, err)
	case <-time.After(12 * time.Second):
		close(release)
		t.Fatal("forced HTTP close did not happen")
	}
	select {
	case <-closed:
		t.Error("storage closed with a running handler")
	default:
	}
	close(release)
	require.ErrorIs(t, <-done, context.DeadlineExceeded)
}

package postgres_test

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/EvgeniyAleksandrov/gophermart/internal/repository/postgres"
	"github.com/stretchr/testify/require"
)

func TestDBStorageBuilderFailure(t *testing.T) {
	builder := postgres.NewDBStorageBuilder(
		"postgres://user:secret@localhost:invalid/db",
		nil,
		singleAttempt(t),
	)
	storage, err := builder.Build(context.Background())
	require.Error(t, err)
	require.Nil(t, storage)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	builder = postgres.NewDBStorageBuilder(
		"host=localhost user=test dbname=test sslmode=disable",
		nil,
		singleAttempt(t),
	)
	storage, err = builder.Build(ctx)
	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, storage)
}

func TestDBStorageClose(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	storage := postgres.NewDBStorage(db, singleAttempt(t))
	mock.ExpectClose()
	require.NoError(t, storage.Close())
	require.NoError(t, storage.Close())
	require.NoError(t, mock.ExpectationsWereMet())
}

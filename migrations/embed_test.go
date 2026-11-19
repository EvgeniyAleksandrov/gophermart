package migrations

import (
	"errors"
	"io"
	"io/fs"
	"testing"

	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/stretchr/testify/require"
)

func TestEmbeddedMigrations(t *testing.T) {
	source, err := iofs.New(NewSource().FS(), ".")
	require.NoError(t, err)
	defer source.Close()

	version, err := source.First()
	require.NoError(t, err)
	for expected := uint(1); expected <= 4; expected++ {
		require.Equal(t, expected, version)
		for _, read := range []func(uint) (io.ReadCloser, string, error){source.ReadUp, source.ReadDown} {
			file, identifier, err := read(version)
			require.NoError(t, err)
			require.NotEmpty(t, identifier)
			contents, err := io.ReadAll(file)
			require.NoError(t, err)
			require.NotEmpty(t, contents)
			require.NoError(t, file.Close())
		}
		version, err = source.Next(version)
		if expected == 4 {
			require.True(t, errors.Is(err, fs.ErrNotExist))
			continue
		}
		require.NoError(t, err)
	}
}

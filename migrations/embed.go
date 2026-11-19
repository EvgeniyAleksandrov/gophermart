package migrations

import (
	"embed"
	"io/fs"
)

//go:embed *.sql
var files embed.FS

// Source provides the SQL migrations embedded in the executable.
type Source struct{}

// NewSource creates an embedded migration source without filesystem access.
func NewSource() *Source { return &Source{} }

// FS returns the read-only migration files, independent of the working directory.
func (s *Source) FS() fs.FS { return files }

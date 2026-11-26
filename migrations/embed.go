package migrations

import (
	"embed"
	"io/fs"
)

//go:embed *.sql
var files embed.FS

type Source struct {
}

func NewSource() *Source { return &Source{} }

func (s *Source) FS() fs.FS { return files }

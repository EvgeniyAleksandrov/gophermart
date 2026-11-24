package service_test

import (
	"strings"
	"testing"

	"github.com/EvgeniyAleksandrov/gophermart/internal/model"
	"github.com/EvgeniyAleksandrov/gophermart/internal/service"
	"github.com/stretchr/testify/require"
)

func TestOrderNumberValidator(t *testing.T) {
	tests := map[string]struct {
		number  string
		wantErr error
	}{
		"zero": {
			number: "0",
		},
		"valid": {
			number: "18",
		},
		"known order": {
			number: "12345678903",
		},
		"long": {
			number: "1" + strings.Repeat("0", 100000) + "8",
		},
		"empty": {
			wantErr: model.ErrEmptyOrderNumber,
		},
		"checksum": {
			number:  "19",
			wantErr: model.ErrInvalidOrderNumber,
		},
		"double zero": {
			number:  "00",
			wantErr: model.ErrInvalidOrderNumber,
		},
		"leading zero": {
			number:  "018",
			wantErr: model.ErrInvalidOrderNumber,
		},
		"space": {
			number:  "1 8",
			wantErr: model.ErrInvalidOrderNumber,
		},
		"minus": {
			number:  "-18",
			wantErr: model.ErrInvalidOrderNumber,
		},
		"letter": {
			number:  "a18",
			wantErr: model.ErrInvalidOrderNumber,
		},
		"unicode": {
			number:  "１８",
			wantErr: model.ErrInvalidOrderNumber,
		},
		"untrimmed": {
			number:  "18\n",
			wantErr: model.ErrInvalidOrderNumber,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			require.ErrorIs(t, service.NewOrderNumberValidator().Validate(tc.number), tc.wantErr)
		})
	}
}

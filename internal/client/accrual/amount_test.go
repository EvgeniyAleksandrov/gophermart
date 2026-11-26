package accrual_test

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/EvgeniyAleksandrov/gophermart/internal/client/accrual"
	"github.com/EvgeniyAleksandrov/gophermart/internal/model"
	"github.com/stretchr/testify/require"
)

func TestAmountParser(t *testing.T) {
	tests := map[string]struct {
		raw     string
		want    int64
		invalid bool
	}{
		"absent": {},
		"null": {
			raw: "null",
		},
		"zero": {
			raw: "0",
		},
		"round down": {
			raw:  "12.344",
			want: 1234,
		},
		"round half up": {
			raw:  "12.345",
			want: 1235,
		},
		"carry": {
			raw:  "12.999",
			want: 1300,
		},
		"tiny": {
			raw: "0.0000001",
		},
		"exponent": {
			raw:  "12345e-3",
			want: 1235,
		},
		"maximum": {
			raw:  "92233720368547758.074",
			want: math.MaxInt64,
		},
		"overflow rounding": {
			raw:     "92233720368547758.075",
			invalid: true,
		},
		"negative tiny": {
			raw:     "-0.0001",
			invalid: true,
		},
		"negative zero": {
			raw: "-0.00",
		},
		"string": {
			raw:     `"12"`,
			invalid: true,
		},
		"bool": {
			raw:     "true",
			invalid: true,
		},
		"object": {
			raw:     "{}",
			invalid: true,
		},
		"array": {
			raw:     "[]",
			invalid: true,
		},
		"invalid number": {
			raw:     "01",
			invalid: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := accrual.NewAmountParser().Parse(json.RawMessage(tc.raw))
			if tc.invalid {
				require.ErrorIs(t, err, model.ErrAccrualInvalidResponse)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

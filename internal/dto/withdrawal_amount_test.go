package dto_test

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/EvgeniyAleksandrov/gophermart/internal/dto"
	"github.com/EvgeniyAleksandrov/gophermart/internal/model"
	"github.com/stretchr/testify/require"
)

func TestWithdrawalAmountParse(t *testing.T) {
	for name, tc := range map[string]struct {
		raw  string
		want int64
		err  error
	}{
		"decimal": {
			raw:  "12.34",
			want: 1234,
		},
		"trailing zero": {
			raw:  "12.340",
			want: 1234,
		},
		"exponent": {
			raw:  "1.234e1",
			want: 1234,
		},
		"signed exponent": {
			raw:  "123400E-4",
			want: 1234,
		},
		"smallest": {
			raw:  "0.01",
			want: 1,
		},
		"max": {
			raw:  "92233720368547758.07",
			want: math.MaxInt64,
		},
		"max exponent": {
			raw:  "9223372036854775807e-2",
			want: math.MaxInt64,
		},
		"overflow": {
			raw: "92233720368547758.08",
			err: model.ErrInvalidWithdrawalAmount,
		},
		"extra precision": {
			raw: "12.345",
			err: model.ErrInvalidWithdrawalAmount,
		},
		"tiny": {
			raw: "0.001",
			err: model.ErrInvalidWithdrawalAmount,
		},
		"zero": {
			raw: "0",
			err: model.ErrInvalidWithdrawalAmount,
		},
		"minus zero": {
			raw: "-0",
			err: model.ErrInvalidWithdrawalAmount,
		},
		"negative": {
			raw: "-1.23",
			err: model.ErrInvalidWithdrawalAmount,
		},
		"huge exponent": {
			raw: "1e9999999999999999999999999999",
			err: model.ErrInvalidWithdrawalAmount,
		},
		"huge negative exponent": {
			raw: "1e-9999999999999999999999999999",
			err: model.ErrInvalidWithdrawalAmount,
		},
		"bounded exponent": {
			raw: "1e99999",
			err: model.ErrInvalidWithdrawalAmount,
		},
		"long mantissa balanced by exponent": {
			raw:  "1" + strings.Repeat("0", 1000) + "e-1000",
			want: 100,
		},
		"long fraction": {
			raw:  "0." + strings.Repeat("0", 1000) + "1e999",
			want: 1,
		},
		"whitespace": {
			raw:  " \n 0.10 \t",
			want: 10,
		},
		"missing": {
			err: dto.ErrInvalidWithdrawalJSON,
		},
		"null": {
			raw: "null",
			err: dto.ErrInvalidWithdrawalJSON,
		},
		"string": {
			raw: `"12.34"`,
			err: dto.ErrInvalidWithdrawalJSON,
		},
		"bool": {
			raw: "true",
			err: dto.ErrInvalidWithdrawalJSON,
		},
		"array": {
			raw: "[]",
			err: dto.ErrInvalidWithdrawalJSON,
		},
		"object": {
			raw: "{}",
			err: dto.ErrInvalidWithdrawalJSON,
		},
		"rational syntax": {
			raw: "1/2",
			err: dto.ErrInvalidWithdrawalJSON,
		},
		"hex": {
			raw: "0x12",
			err: dto.ErrInvalidWithdrawalJSON,
		},
		"leading zero": {
			raw: "01",
			err: dto.ErrInvalidWithdrawalJSON,
		},
		"plus": {
			raw: "+1",
			err: dto.ErrInvalidWithdrawalJSON,
		},
		"two numbers": {
			raw: "1 2",
			err: dto.ErrInvalidWithdrawalJSON,
		},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := dto.NewWithdrawalAmount().Parse(json.RawMessage(tc.raw))
			require.ErrorIs(t, err, tc.err)
			require.Equal(t, tc.want, got)
		})
	}
}

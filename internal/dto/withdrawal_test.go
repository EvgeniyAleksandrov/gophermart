package dto_test

import (
	"strings"
	"testing"

	"github.com/EvgeniyAleksandrov/gophermart/internal/dto"
	"github.com/EvgeniyAleksandrov/gophermart/internal/model"
	"github.com/stretchr/testify/require"
)

func TestWithdrawalRequestDecode(t *testing.T) {
	for name, tc := range map[string]struct {
		body  string
		order string
		sum   int64
		err   error
	}{
		"valid": {
			body:  `{"order":"18","sum":12.340}`,
			order: "18",
			sum:   1234,
		},
		"unknown and duplicates": {
			body:  `{"order":"0","order":"18","sum":0,"sum":1.234e1,"user_id":99}`,
			order: "18",
			sum:   1234,
		},
		"empty order retained": {
			body: `{"order":"","sum":1}`,
			sum:  100,
		},
		"spaces retained": {
			body:  `{"order":" 18 ","sum":1}`,
			order: " 18 ",
			sum:   100,
		},
		"empty": {
			err: dto.ErrInvalidWithdrawalJSON,
		},
		"null": {
			body: "null",
			err:  dto.ErrInvalidWithdrawalJSON,
		},
		"array": {
			body: "[]",
			err:  dto.ErrInvalidWithdrawalJSON,
		},
		"missing order before invalid amount": {
			body: `{"sum":0}`,
			err:  dto.ErrInvalidWithdrawalJSON,
		},
		"null order": {
			body: `{"order":null,"sum":1}`,
			err:  dto.ErrInvalidWithdrawalJSON,
		},
		"number order": {
			body: `{"order":18,"sum":1}`,
			err:  dto.ErrInvalidWithdrawalJSON,
		},
		"missing sum": {
			body: `{"order":"18"}`,
			err:  dto.ErrInvalidWithdrawalJSON,
		},
		"null sum": {
			body: `{"order":"18","sum":null}`,
			err:  dto.ErrInvalidWithdrawalJSON,
		},
		"invalid amount": {
			body: `{"order":"18","sum":0}`,
			err:  model.ErrInvalidWithdrawalAmount,
		},
		"second JSON before invalid amount": {
			body: `{"order":"18","sum":0} {}`,
			err:  dto.ErrInvalidWithdrawalJSON,
		},
		"trailing garbage": {
			body: `{"order":"18","sum":1} x`,
			err:  dto.ErrInvalidWithdrawalJSON,
		},
		"malformed": {
			body: `{"order":"18","sum":1`,
			err:  dto.ErrInvalidWithdrawalJSON,
		},
	} {
		t.Run(name, func(t *testing.T) {
			request := dto.WithdrawalRequest{Order: "previous", Sum: 100}
			err := request.Decode(strings.NewReader(tc.body))
			require.ErrorIs(t, err, tc.err)
			require.Equal(t, tc.order, request.Order)
			require.Equal(t, tc.sum, request.Sum)
		})
	}
}

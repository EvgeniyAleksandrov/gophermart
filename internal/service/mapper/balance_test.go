package mapper_test

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/EvgeniyAleksandrov/gophermart/internal/model"
	"github.com/EvgeniyAleksandrov/gophermart/internal/service/mapper"
	"github.com/stretchr/testify/require"
)

func TestBalance(t *testing.T) {
	for name, tc := range map[string]struct {
		balance model.Balance
		json    string
		invalid bool
	}{
		"zero": {
			json: `{"current":0.00,"withdrawn":0.00}`,
		},
		"exact large": {
			balance: model.Balance{Current: math.MaxInt64, Withdrawn: 12345},
			json:    `{"current":92233720368547758.07,"withdrawn":123.45}`,
		},
		"negative current": {
			balance: model.Balance{Current: -1},
			invalid: true,
		},
		"negative withdrawn": {
			balance: model.Balance{Withdrawn: -1},
			invalid: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := mapper.NewBalance().Present(tc.balance)
			if tc.invalid {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			raw, err := json.Marshal(got)
			require.NoError(t, err)
			require.Equal(t, tc.json, string(raw))
		})
	}
}

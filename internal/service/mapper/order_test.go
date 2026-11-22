package mapper_test

import (
	"encoding/json"
	"math"
	"testing"
	"time"

	"github.com/EvgeniyAleksandrov/gophermart/internal/model"
	"github.com/EvgeniyAleksandrov/gophermart/internal/service/mapper"
	"github.com/stretchr/testify/require"
)

func TestOrder(t *testing.T) {
	zero, fraction, whole, maximum, negative := int64(0), int64(50050), int64(50000), int64(math.MaxInt64), int64(-1)
	tests := map[string]struct {
		status      model.OrderStatus
		accrual     *int64
		wantAccrual string
		wantErr     bool
	}{
		"new": {
			status: model.OrderStatusNew,
		},
		"processing": {
			status: model.OrderStatusProcessing,
		},
		"invalid zero": {
			status:      model.OrderStatusInvalid,
			accrual:     &zero,
			wantAccrual: ",\"accrual\":0.00",
		},
		"processed zero": {
			status:      model.OrderStatusProcessed,
			accrual:     &zero,
			wantAccrual: ",\"accrual\":0.00",
		},
		"fraction": {
			status:      model.OrderStatusProcessed,
			accrual:     &fraction,
			wantAccrual: ",\"accrual\":500.50",
		},
		"whole": {
			status:      model.OrderStatusProcessed,
			accrual:     &whole,
			wantAccrual: ",\"accrual\":500.00",
		},
		"maximum": {
			status:      model.OrderStatusProcessed,
			accrual:     &maximum,
			wantAccrual: ",\"accrual\":92233720368547758.07",
		},
		"negative": {
			status:  model.OrderStatusProcessed,
			accrual: &negative,
			wantErr: true,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			orders := []model.Order{{
				ID:         42,
				UserID:     7,
				Number:     "18",
				Status:     tc.status,
				Accrual:    tc.accrual,
				UploadedAt: time.Date(2026, 11, 22, 12, 30, 1, 123, time.FixedZone("MSK", 3*60*60)),
			}}
			result, err := mapper.NewOrder().Present(orders)
			if tc.wantErr {
				require.Error(t, err)
				require.Nil(t, result)
				return
			}

			require.NoError(t, err)
			data, err := json.Marshal(result)
			require.NoError(t, err)
			require.Equal(t, `[{"number":"18","status":"`+string(tc.status)+`"`+tc.wantAccrual+`,"uploaded_at":"2026-11-22T09:30:01Z"}]`, string(data))
		})
	}
}

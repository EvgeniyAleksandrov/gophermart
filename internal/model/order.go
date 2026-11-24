package model

import "time"

type OrderStatus string

const (
	// OrderStatusNew marks an order awaiting processing.
	OrderStatusNew OrderStatus = "NEW"
	// OrderStatusProcessing marks an order being calculated.
	OrderStatusProcessing OrderStatus = "PROCESSING"
	// OrderStatusInvalid marks a rejected order with zero accrual.
	OrderStatusInvalid OrderStatus = "INVALID"
	// OrderStatusProcessed marks an order with a final accrual.
	OrderStatusProcessed OrderStatus = "PROCESSED"
)

type Order struct {
	ID         int64
	UserID     int64
	Number     string
	Status     OrderStatus
	Accrual    *int64
	UploadedAt time.Time
}

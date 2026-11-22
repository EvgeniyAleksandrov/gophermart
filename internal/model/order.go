package model

import "time"

// OrderStatus identifies the processing state of an uploaded order.
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

// Order is a stored order. Accrual is in hundredths, or nil before completion.
// UploadedAt is assigned by PostgreSQL; internal IDs are not transport fields.
type Order struct {
	ID         int64
	UserID     int64
	Number     string
	Status     OrderStatus
	Accrual    *int64
	UploadedAt time.Time
}

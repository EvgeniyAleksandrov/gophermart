package service

import "github.com/EvgeniyAleksandrov/gophermart/internal/model"

// OrderNumberValidator validates ASCII order numbers using the Luhn checksum.
type OrderNumberValidator struct{}

// NewOrderNumberValidator constructs a stateless validator.
func NewOrderNumberValidator() *OrderNumberValidator {
	return &OrderNumberValidator{}
}

// Validate accepts a trimmed number, allowing only a single leading zero.
// The running checksum is bounded regardless of the number's length.
func (v *OrderNumberValidator) Validate(number string) error {
	if number == "" {
		return model.ErrEmptyOrderNumber
	}

	if len(number) > 1 && number[0] == '0' {
		return model.ErrInvalidOrderNumber
	}

	sum := 0
	double := false
	for i := len(number) - 1; i >= 0; i-- {
		if number[i] < '0' || number[i] > '9' {
			return model.ErrInvalidOrderNumber
		}

		digit := int(number[i] - '0')
		if double {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}

		sum = (sum + digit) % 10
		double = !double
	}

	if sum != 0 {
		return model.ErrInvalidOrderNumber
	}

	return nil
}

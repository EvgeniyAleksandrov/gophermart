//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -source=interfaces.go -destination=interfaces_mock_test.go -package=service_test

package service

type OrderNumberChecker interface {
	Validate(number string) error
}

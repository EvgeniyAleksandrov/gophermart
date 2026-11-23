package model

// Balance holds available and previously withdrawn amounts in hundredths.
type Balance struct {
	Current   int64
	Withdrawn int64
}

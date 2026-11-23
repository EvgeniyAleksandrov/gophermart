package strategy

import "errors"

// SQLState matches configured SQLSTATE codes without depending on a SQL driver.
type SQLState struct{ codes map[string]struct{} }

// NewSQLState copies the allowed codes so subsequent caller changes are isolated.
func NewSQLState(codes []string) *SQLState {
	checker := &SQLState{codes: make(map[string]struct{}, len(codes))}
	for _, code := range codes {
		checker.codes[code] = struct{}{}
	}

	return checker
}

// Matches finds a SQLState error through errors.Is/As-compatible wrappers.
func (s *SQLState) Matches(err error) bool {
	var state interface{ SQLState() string }
	if !errors.As(err, &state) {
		return false
	}
	_, ok := s.codes[state.SQLState()]
	return ok
}

package strategy

import "errors"

type SQLState struct {
	codes map[string]struct{}
}

func NewSQLState(codes []string) *SQLState {
	checker := &SQLState{codes: make(map[string]struct{}, len(codes))}

	for _, code := range codes {
		checker.codes[code] = struct{}{}
	}

	return checker
}

func (s *SQLState) Matches(err error) bool {
	var state interface {
		SQLState() string
	}
	if !errors.As(err, &state) {
		return false
	}

	_, ok := s.codes[state.SQLState()]

	return ok
}

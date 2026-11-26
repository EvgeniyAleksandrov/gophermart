package interfaces

import "context"

type Retryer interface {
	Do(context.Context, func() error) error
}

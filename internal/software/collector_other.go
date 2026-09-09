//go:build !windows

package software

import (
	"context"
	"errors"
)

func collectWindows(context.Context) ([]Installation, error) {
	return nil, errors.New("software registry collection is only supported on Windows")
}

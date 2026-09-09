//go:build !windows

package inventory

import (
	"context"
	"errors"
)

func Collect(context.Context) (Hardware, error) {
	return Hardware{}, errors.New("hardware inventory is only supported on Windows")
}

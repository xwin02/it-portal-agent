package software

import "context"

func Collect(ctx context.Context) ([]Installation, error) { return collectWindows(ctx) }

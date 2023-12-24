package context

import (
	"context"
	"time"
)

type Key int

const (
	_ Key = iota
	DBNameKey
	HTTPTimeoutKey
	UserInfoKey
)

func WithTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	timeout, _ := ctx.Value(HTTPTimeoutKey).(int)
	return context.WithTimeout(ctx, time.Second*time.Duration(timeout))
}

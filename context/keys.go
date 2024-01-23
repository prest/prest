package context

import (
	"context"
	"time"
)

type Key int

const (
	_              Key = iota
	DBNameKey          // DBNameKey is the key for the database name
	HTTPTimeoutKey     // HTTPTimeoutKey is the key for the http timeout
	UserInfoKey        // UserInfoKey is the key for the user info
)

// WithTimeout returns a context with timeout
// if timeout is not setted, will be used 60 seconds
func WithTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	timeout, _ := ctx.Value(HTTPTimeoutKey).(int)
	if timeout == 0 {
		timeout = 60
	}
	return context.WithTimeout(ctx, time.Second*time.Duration(timeout))
}

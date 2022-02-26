package logger

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
)

type ctxKey int

const (
	ctxKeyLog = iota
)

func Entry(ctx context.Context) *logrus.Entry {
	v := ctx.Value(ctxKeyLog)
	var e *logrus.Entry
	e, ok := v.(*logrus.Entry)
	if !ok {
		err := fmt.Errorf("not a *logrus.Entry: %T", v)
		panic(err)
	}
	return e
}

func WithLogEntry(ctx context.Context, e *logrus.Entry) context.Context {
	return context.WithValue(ctx, ctxKeyLog, e)
}

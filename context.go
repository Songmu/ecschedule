package ecschedule

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
)

type contextKey string

const appKey contextKey = "app"

type app struct {
	Config    *Config
	AccountID string
	AwsConf   aws.Config
}

func setApp(ctx context.Context, a *app) context.Context {
	return context.WithValue(ctx, appKey, a)
}

func getApp(ctx context.Context) *app {
	iface := ctx.Value(appKey)
	if a, ok := iface.(*app); ok {
		return a
	}
	return nil
}

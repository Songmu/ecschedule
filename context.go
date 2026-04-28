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
	ExtStr    map[string]string
	ExtCode   map[string]string
}

func (a *app) loadConfigOptions() []LoadConfigOption {
	var opts []LoadConfigOption
	if len(a.ExtStr) > 0 {
		opts = append(opts, WithExtStr(a.ExtStr))
	}
	if len(a.ExtCode) > 0 {
		opts = append(opts, WithExtCode(a.ExtCode))
	}
	return opts
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

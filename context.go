package ecsched

import "context"

type contextKey string

const configKey contextKey = "config"

func setConfig(ctx context.Context, c *Config) context.Context {
	return context.WithValue(ctx, configKey, c)
}

func getConfig(ctx context.Context) *Config {
	iface := ctx.Value(configKey)
	if c, ok := iface.(*Config); ok {
		return c
	}
	return nil
}

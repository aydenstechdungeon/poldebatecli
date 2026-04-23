package client

import (
	"context"
	"fmt"
	"sync"

	"golang.org/x/time/rate"
)

type RateLimiter struct {
	limiters   sync.Map
	defaultRPS float64
	global     *rate.Limiter
}

func NewRateLimiter(rps float64) *RateLimiter {
	return &RateLimiter{
		defaultRPS: rps,
		global:     rate.NewLimiter(rate.Limit(rps), 1),
	}
}

func (r *RateLimiter) Wait(ctx context.Context, model string) error {
	if err := r.global.Wait(ctx); err != nil {
		return fmt.Errorf("global rate limit wait: %w", err)
	}
	limiter, _ := r.limiters.LoadOrStore(model, rate.NewLimiter(rate.Limit(r.defaultRPS), 1))
	limiterVal, ok := limiter.(*rate.Limiter)
	if !ok {
		return fmt.Errorf("invalid limiter type for model %s", model)
	}
	if err := limiterVal.Wait(ctx); err != nil {
		return fmt.Errorf("rate limit wait for model %s: %w", model, err)
	}
	return nil
}

func (r *RateLimiter) WaitDefault(ctx context.Context) error {
	return r.Wait(ctx, "_default")
}

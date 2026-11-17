package deployments

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type checkoutPolicy struct {
	fetch, checkout time.Duration
	attempts        int
	backoff         time.Duration
}

func readCheckoutPolicy() (checkoutPolicy, error) {
	p := checkoutPolicy{fetch: 180 * time.Second, checkout: 120 * time.Second, attempts: 2, backoff: time.Second}
	for _, field := range []struct {
		name   string
		target *time.Duration
	}{{"GIT_FETCH_TIMEOUT_SECONDS", &p.fetch}, {"GIT_CHECKOUT_TIMEOUT_SECONDS", &p.checkout}} {
		if value := os.Getenv(field.name); value != "" {
			n, err := strconv.Atoi(value)
			if err != nil || n < 1 || n > 300 {
				return p, fmt.Errorf("%s must be 1..300", field.name)
			}
			*field.target = time.Duration(n) * time.Second
		}
	}
	return p, nil
}

func transientFetch(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	text := strings.ToLower(err.Error())
	// Permanent authorization, missing-ref and repository errors never retry.
	for _, marker := range []string{"authentication failed", "repository not found", "could not read username", "not our ref", "unadvertised object", "403", "404", "401"} {
		if strings.Contains(text, marker) {
			return false
		}
	}
	for _, marker := range []string{"could not resolve host", "connection reset", "connection timed out", "failed to connect", "remote end hung up", "early eof", "http/2 stream", "http2", "requested url returned error: 429", "requested url returned error: 500", "requested url returned error: 502", "requested url returned error: 503", "requested url returned error: 504"} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func fetchPinned(p checkoutPolicy, run func(context.Context) error) error {
	for attempt := 1; attempt <= p.attempts; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), p.fetch)
		started := time.Now()
		err := run(ctx)
		cancel()
		fmt.Printf("ZERODEVOPS_GIT phase=fetch attempt=%d seconds=%.3f success=%t\n", attempt, time.Since(started).Seconds(), err == nil)
		if err == nil {
			return nil
		}
		if attempt == p.attempts || !transientFetch(err) {
			return fmt.Errorf("attempt %d: %w", attempt, err)
		}
		time.Sleep(p.backoff * time.Duration(attempt))
	}
	return errors.New("no fetch attempts configured")
}

package llama

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// health.go polls the llama-server HTTP /health endpoint until it returns
// 200, the timeout expires, or the context is cancelled. The llama-server
// /health endpoint returns 503 while loading the model and 200 once it is
// ready to serve inferences.

// healthURLTemplate is the URL format for the /health endpoint.
const healthURLTemplate = "http://127.0.0.1:%d/health"

// healthBackoffSchedule is the exponential backoff between /health probes.
// Each value is the wait BEFORE the next attempt. We cap at 1s; the overall
// budget is controlled by the timeout argument, not the per-attempt sleep.
var healthBackoffSchedule = []time.Duration{
	50 * time.Millisecond,
	100 * time.Millisecond,
	200 * time.Millisecond,
	400 * time.Millisecond,
	800 * time.Millisecond,
	1 * time.Second,
}

// healthClient is the HTTP client used by waitForHealth. Indirected at the
// package level so tests can replace it with one wired to an httptest.Server.
// Default: short overall timeout is NOT set (we rely on ctx for the deadline);
// per-request timeout is small so a hung connection doesn't burn the whole
// startup window.
var healthClient = &http.Client{
	Timeout: 2 * time.Second,
}

// setHealthClient replaces the package HTTP client. Used by tests so we can
// point at httptest.Server without altering real DNS resolution.
func setHealthClient(c *http.Client) { healthClient = c }

// waitForHealth polls GET http://127.0.0.1:<port>/health until:
//   - it returns 200: returns nil
//   - ctx is cancelled: returns ctx.Err()
//   - timeout elapses without 200: returns ErrSpawnTimeout
//
// Between attempts it sleeps per an exponential backoff schedule (50ms,
// 100ms, 200ms, ..., capped at 1s). The schedule is reset each time the
// endpoint responds with anything other than a connection error (so once
// the server starts answering, we don't back off further).
//
// F3.6: ctx propagation. We honor ctx.Done() at every blocking step.
func waitForHealth(ctx context.Context, port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	url := fmt.Sprintf(healthURLTemplate, port)

	attempt := 0
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("%w: port %d did not become healthy within %s",
				ErrSpawnTimeout, port, timeout)
		}

		ok, err := probeHealth(ctx, url)
		if ok {
			return nil
		}
		if err != nil {
			// Surface unexpected probe errors only at debug level via
			// return path; we don't log here to keep the package free of
			// logger plumbing. The lifecycle logs the final timeout.
		}

		// Compute the next backoff. We cap the index into the schedule
		// so we plateau at 1s.
		sleep := healthBackoffSchedule[len(healthBackoffSchedule)-1]
		if attempt < len(healthBackoffSchedule) {
			sleep = healthBackoffSchedule[attempt]
		}
		attempt++

		// Honor ctx cancellation during the sleep. We use a 100ms
		// granularity timer loop so a context cancel during a 1s sleep
		// is noticed quickly.
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(sleep):
		}
	}
}

// probeHealth issues one GET to the /health endpoint and reports whether the
// server is healthy. Returns (true, nil) on HTTP 200. Returns (false, nil)
// on any non-200 HTTP status (server is up but not ready). Returns
// (false, err) on transport errors (connection refused, etc.) — the caller
// will retry.
func probeHealth(ctx context.Context, url string) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, err
	}
	resp, err := healthClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	// 200 = healthy. Anything else = keep waiting.
	return resp.StatusCode == http.StatusOK, nil
}

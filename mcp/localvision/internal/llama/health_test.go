package llama

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// health_test.go exercises waitForHealth and probeHealth. We stand up an
// httptest.Server that returns 503 until flipped to 200, then verify
// waitForHealth eventually succeeds.

// TestWaitForHealthImmediate200: server returns 200 on the first probe.
// waitForHealth returns nil without backing off.
func TestWaitForHealthImmediate200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	port := srv.Listener.Addr().(*net.TCPAddr).Port

	setHealthClient(&http.Client{Timeout: 2 * time.Second})
	t.Cleanup(func() { setHealthClient(&http.Client{}) })

	err := waitForHealth(context.Background(), port, 5*time.Second)
	require.NoError(t, err)
}

// TestWaitForHealthEventuallyReady: server returns 503 the first two probes,
// then 200. waitForHealth should succeed within the timeout.
func TestWaitForHealthEventuallyReady(t *testing.T) {
	var count int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&count, 1)
		if n < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	port := srv.Listener.Addr().(*net.TCPAddr).Port

	setHealthClient(&http.Client{Timeout: 2 * time.Second})
	t.Cleanup(func() { setHealthClient(&http.Client{}) })

	err := waitForHealth(context.Background(), port, 5*time.Second)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, atomic.LoadInt32(&count), int32(3))
}

// TestWaitForHealthTimeout: server always returns 503; waitForHealth times
// out and returns ErrSpawnTimeout.
func TestWaitForHealthTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	port := srv.Listener.Addr().(*net.TCPAddr).Port

	setHealthClient(&http.Client{Timeout: 100 * time.Millisecond})
	t.Cleanup(func() { setHealthClient(&http.Client{})})

	start := time.Now()
	err := waitForHealth(context.Background(), port, 300*time.Millisecond)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrSpawnTimeout), "got %v", err)
	elapsed := time.Since(start)
	assert.Less(t, elapsed, 2*time.Second, "should not wait significantly past timeout")
}

// TestWaitForHealthCtxCancelled: ctx already cancelled on entry returns
// ctx.Err() immediately.
func TestWaitForHealthCtxCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := waitForHealth(ctx, 12345, 5*time.Second)
	require.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled))
}

// TestWaitForHealthConnectionRefused: nothing is listening on the port.
// waitForHealth should retry then time out (or return ErrSpawnTimeout).
func TestWaitForHealthConnectionRefused(t *testing.T) {
	// Grab a port then close it.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := l.Addr().(*net.TCPAddr).Port
	require.NoError(t, l.Close())

	setHealthClient(&http.Client{Timeout: 100 * time.Millisecond})
	t.Cleanup(func() { setHealthClient(&http.Client{}) })

	err = waitForHealth(context.Background(), port, 300*time.Millisecond)
	require.Error(t, err)
	// Either ErrSpawnTimeout or some other transient — both are fine.
}

// TestProbeHealth: cover probeHealth directly.
func TestProbeHealth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	port := srv.Listener.Addr().(*net.TCPAddr).Port
	url := srv.URL + "/health"

	setHealthClient(&http.Client{Timeout: 2 * time.Second})
	t.Cleanup(func() { setHealthClient(&http.Client{}) })

	ok, err := probeHealth(context.Background(), url)
	require.NoError(t, err)
	assert.True(t, ok)
	_ = port
}

// TestProbeHealthNon200: a 503 returns (false, nil) — caller should retry.
func TestProbeHealthNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	setHealthClient(&http.Client{Timeout: 2 * time.Second})
	t.Cleanup(func() { setHealthClient(&http.Client{}) })

	ok, err := probeHealth(context.Background(), srv.URL+"/health")
	require.NoError(t, err)
	assert.False(t, ok)
}

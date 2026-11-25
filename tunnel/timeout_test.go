// Copyright (c) 2023 RightScale, Inc. - see LICENSE

package tunnel

import (
	"net/http"
	"sync"
	"testing"
	"time"
)

func setupTimeoutTest(t *testing.T) *tunnelTestEnv {
	t.Helper()
	return setupTunnelTest(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/delayed" {
			time.Sleep(5 * time.Second)
			_, _ = w.Write([]byte("This response should never be seen"))
		}
	}), "-httptimeout", "2")
}

func TestTokenRequestTimeout(t *testing.T) {
	env := setupTimeoutTest(t)

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	start := time.Now()
	resp, err := client.Get(env.wstunURL + "/_token/" + env.token + "/delayed")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Expected 504 response, got error after %v: %v", elapsed, err)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusGatewayTimeout {
		t.Fatalf("Expected 504 Gateway Timeout, got status: %d after %v", resp.StatusCode, elapsed)
	}

	if elapsed < 1500*time.Millisecond {
		t.Fatalf("Timeout occurred too early: %v (expected ~2s)", elapsed)
	}
	if elapsed > 4*time.Second {
		t.Fatalf("Timeout occurred too late: %v (expected ~2s)", elapsed)
	}
}

func TestTokenRequestConcurrentTimeouts(t *testing.T) {
	env := setupTimeoutTest(t)

	const numRequests = 5
	var wg sync.WaitGroup
	type result struct {
		status  int
		elapsed time.Duration
		err     error
	}
	results := make([]result, numRequests)

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	wg.Add(numRequests)
	globalStart := time.Now()
	for i := range numRequests {
		go func(idx int) {
			defer wg.Done()
			reqStart := time.Now()
			resp, err := client.Get(env.wstunURL + "/_token/" + env.token + "/delayed")
			results[idx] = result{elapsed: time.Since(reqStart), err: err}
			if err == nil {
				results[idx].status = resp.StatusCode
				_ = resp.Body.Close()
			}
		}(i)
	}
	wg.Wait()
	totalElapsed := time.Since(globalStart)

	for i, r := range results {
		if r.err != nil {
			t.Errorf("Request %d: expected 504 response, got error after %v: %v", i, r.elapsed, r.err)
			continue
		}
		if r.status != http.StatusGatewayTimeout {
			t.Errorf("Request %d: expected 504, got %d after %v", i, r.status, r.elapsed)
		}
		if r.elapsed < 1500*time.Millisecond {
			t.Errorf("Request %d: timeout too early: %v (expected ~2s)", i, r.elapsed)
		}
		if r.elapsed > 5*time.Second {
			t.Errorf("Request %d: timeout too late: %v (expected ~2s)", i, r.elapsed)
		}
	}

	if totalElapsed > 6*time.Second {
		t.Errorf("All requests took too long: %v (expected ~2-4s)", totalElapsed)
	}
}

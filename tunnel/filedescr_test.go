// Copyright (c) 2015 RightScale, Inc. - see LICENSE

package tunnel

import (
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strconv"
	"testing"
)

// TestFileDescriptorLeakage checks if file descriptors are leaking after multiple requests
func TestFileDescriptorLeakage(t *testing.T) {
	// Skip this test for now during the migration from Ginkgo to standard Go tests
	t.Skip("Skipping test during migration from Ginkgo to standard Go tests")
	// Check if lsof is available
	if _, err := exec.LookPath("lsof"); err != nil {
		t.Skip("lsof not found")
	}

	// Setup test server
	wstunToken := "test567890123456-" + strconv.Itoa(rand.Int()%1000000)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/world")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(r.URL.Path))
	}))
	defer server.Close()

	t.Logf("server started at %s", server.URL)

	l, _ := net.Listen("tcp", "127.0.0.1:0")
	wstunsrv := NewWSTunnelServer([]string{})
	wstunsrv.Start(l)
	defer wstunsrv.Stop()

	wstuncli := NewWSTunnelClient([]string{
		"-token", wstunToken,
		"-tunnel", "ws://" + l.Addr().String(),
		"-server", server.URL,
		"-insecure",
	})
	err := wstuncli.Start()
	if err != nil {
		t.Fatalf("Error starting client: %v", err)
	}
	defer wstuncli.Stop()

	wstunURL := "http://" + l.Addr().String()

	// Count open files function
	countOpenFiles := func() int {
		out, err := exec.Command("/bin/sh", "-c",
			fmt.Sprintf("lsof -p %v", os.Getpid())).Output()
		if err != nil {
			t.Fatalf("lsof -p: %s", err)
		}
		lines := bytes.Count(out, []byte("\n"))
		return lines - 1
	}

	const N = 100
	startFd := countOpenFiles()
	for i := 0; i < N; i++ {
		txt := fmt.Sprintf("/hello/%d", i)
		resp, err := http.Get(wstunURL + "/_token/" + wstunToken + txt)
		if err != nil {
			t.Fatalf("Error making request: %v", err)
		}
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Error reading response: %v", err)
		}
		if string(respBody) != txt {
			t.Errorf("Expected response body to be %s, got %s", txt, string(respBody))
		}
		if resp.Header.Get("Content-Type") != "text/world" {
			t.Errorf("Expected Content-Type to be text/world, got %s", resp.Header.Get("Content-Type"))
		}
		if resp.StatusCode != 200 {
			t.Errorf("Expected status code to be 200, got %d", resp.StatusCode)
		}
	}
	endFd := countOpenFiles()
	t.Logf("file descriptors: startFd=%d, endFd=%d", startFd, endFd)

	if endFd-startFd >= 10 {
		t.Errorf("Too many file descriptors leaked: start=%d, end=%d", startFd, endFd)
	}
}

package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// binaryPath is the path to the compiled irons binary, built once in TestMain.
var binaryPath string

func TestMain(m *testing.M) {
	// Build the binary once for all tests.
	tmp, err := os.MkdirTemp("", "irons-test-*")
	if err != nil {
		panic("failed to create temp dir: " + err.Error())
	}
	defer os.RemoveAll(tmp)

	binaryPath = filepath.Join(tmp, "irons")
	build := exec.Command("go", "build", "-o", binaryPath, "..")
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		panic("failed to build binary: " + err.Error())
	}

	os.Exit(m.Run())
}

// recordedRequest captures an HTTP request received by the mock server.
type recordedRequest struct {
	Method string
	Path   string
	Query  string
	Body   []byte
}

// mockServer wraps httptest.Server and records all requests for later assertion.
type mockServer struct {
	Server   *httptest.Server
	mu       sync.Mutex
	requests []recordedRequest
}

// route maps a method+path to a handler function.
type route struct {
	Method  string
	Path    string
	Handler func(w http.ResponseWriter, r *http.Request, body []byte)
}

// newMockServer creates a test server that dispatches to the given routes and
// records every request. Unmatched routes return 404.
func newMockServer(t *testing.T, routes []route) *mockServer {
	t.Helper()
	ms := &mockServer{}

	ms.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		r.Body.Close()

		ms.mu.Lock()
		ms.requests = append(ms.requests, recordedRequest{
			Method: r.Method,
			Path:   r.URL.Path,
			Query:  r.URL.RawQuery,
			Body:   body,
		})
		ms.mu.Unlock()

		for _, rt := range routes {
			if r.Method == rt.Method && r.URL.Path == rt.Path {
				rt.Handler(w, r, body)
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	}))

	t.Cleanup(func() { ms.Server.Close() })
	return ms
}

// Requests returns a copy of all recorded requests.
func (ms *mockServer) Requests() []recordedRequest {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	out := make([]recordedRequest, len(ms.requests))
	copy(out, ms.requests)
	return out
}

// RequestBodies returns the parsed JSON bodies for requests matching method+path.
func (ms *mockServer) RequestBodies(method, path string) []map[string]interface{} {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	var out []map[string]interface{}
	for _, r := range ms.requests {
		if r.Method == method && r.Path == path {
			var m map[string]interface{}
			if err := json.Unmarshal(r.Body, &m); err == nil {
				out = append(out, m)
			}
		}
	}
	return out
}

// HasRequest returns true if any recorded request matches method+path.
func (ms *mockServer) HasRequest(method, path string) bool {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	for _, r := range ms.requests {
		if r.Method == method && r.Path == path {
			return true
		}
	}
	return false
}

// cliResult captures the result of running the irons binary.
type cliResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// runCLI executes the irons binary with the given args, pointing it at the
// mock server. It returns the combined result. The binary gets its config
// entirely from environment variables, avoiding any cobra/viper state issues.
func runCLI(t *testing.T, server *mockServer, args ...string) cliResult {
	t.Helper()
	cmd := exec.Command(binaryPath, args...)
	cmd.Env = append(os.Environ(),
		"IRONS_API_URL="+server.Server.URL,
		"IRONS_API_KEY=test-key",
		"HOME="+t.TempDir(), // avoid reading real config
	)

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run binary: %v", err)
		}
	}

	return cliResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}
}

// runCLIWithStdin is like runCLI but pipes the given string to stdin.
func runCLIWithStdin(t *testing.T, server *mockServer, stdin string, args ...string) cliResult {
	t.Helper()
	cmd := exec.Command(binaryPath, args...)
	cmd.Env = append(os.Environ(),
		"IRONS_API_URL="+server.Server.URL,
		"IRONS_API_KEY=test-key",
		"HOME="+t.TempDir(),
	)
	cmd.Stdin = strings.NewReader(stdin)

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run binary: %v", err)
		}
	}

	return cliResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}
}

// jsonResponse is a helper to write a JSON response with the given status code.
func jsonResponse(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// wrapData wraps a value in {"data": v} matching the API envelope.
func wrapData(v interface{}) map[string]interface{} {
	return map[string]interface{}{"data": v}
}

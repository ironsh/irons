package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/ironsh/irons/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sampleSecret returns a sample Secret for testing.
func sampleSecret() api.Secret {
	comment := ""
	return api.Secret{
		ID:         "sec_m4xk9wp2",
		Name:       "github-main",
		EnvVar:     "GITHUB_TOKEN",
		Hosts:      []string{"*"},
		ProxyValue: "IRONSH_PROXY_github-main",
		Comment:    &comment,
		CreatedAt:  "2026-03-04T12:00:00Z",
		UpdatedAt:  "2026-03-04T12:00:00Z",
	}
}

func secretsPostRoute(secret api.Secret) route {
	return route{"POST", "/secrets", func(w http.ResponseWriter, r *http.Request, body []byte) {
		jsonResponse(w, http.StatusCreated, wrapData(secret))
	}}
}

func secretsListRoute(secrets ...api.Secret) route {
	return route{"GET", "/secrets", func(w http.ResponseWriter, r *http.Request, body []byte) {
		jsonResponse(w, http.StatusOK, api.ListSecretsResponse{Data: secrets})
	}}
}

func secretsGetRoute(id string, secret api.Secret) route {
	return route{"GET", "/secrets/" + id, func(w http.ResponseWriter, r *http.Request, body []byte) {
		jsonResponse(w, http.StatusOK, wrapData(secret))
	}}
}

func secretsPatchRoute(id string, secret api.Secret) route {
	return route{"PATCH", "/secrets/" + id, func(w http.ResponseWriter, r *http.Request, body []byte) {
		jsonResponse(w, http.StatusOK, wrapData(secret))
	}}
}

func secretsDeleteRoute(id string) route {
	return route{"DELETE", "/secrets/" + id, func(w http.ResponseWriter, r *http.Request, body []byte) {
		w.WriteHeader(http.StatusNoContent)
	}}
}

// --- secrets add ---

func TestSecretsAdd_AllFlags(t *testing.T) {
	s := sampleSecret()
	ms := newMockServer(t, []route{secretsPostRoute(s)})

	res := runCLI(t, ms, "secrets", "add",
		"--name", "github-main",
		"--env-var", "GITHUB_TOKEN",
		"--secret", "ghp_abc123",
	)
	require.Equal(t, 0, res.ExitCode, res.Stderr)
	assert.Contains(t, res.Stdout, "IRONSH_PROXY_github-main")
	assert.Contains(t, res.Stdout, "GITHUB_TOKEN")

	bodies := ms.RequestBodies("POST", "/secrets")
	require.Len(t, bodies, 1)
	assert.Equal(t, "github-main", bodies[0]["name"])
	assert.Equal(t, "ghp_abc123", bodies[0]["secret"])
	assert.Equal(t, "GITHUB_TOKEN", bodies[0]["env_var"])
}

func TestSecretsAdd_WithHosts(t *testing.T) {
	s := sampleSecret()
	s.Hosts = []string{"api.github.com", "*.github.com"}
	ms := newMockServer(t, []route{secretsPostRoute(s)})

	res := runCLI(t, ms, "secrets", "add",
		"--name", "github-main",
		"--env-var", "GITHUB_TOKEN",
		"--secret", "ghp_abc123",
		"--host", "api.github.com",
		"--host", "*.github.com",
	)
	require.Equal(t, 0, res.ExitCode, res.Stderr)
	assert.Contains(t, res.Stdout, "api.github.com, *.github.com")

	bodies := ms.RequestBodies("POST", "/secrets")
	require.Len(t, bodies, 1)
	assert.Equal(t, []interface{}{"api.github.com", "*.github.com"}, bodies[0]["hosts"])
}

func TestSecretsAdd_WithoutHost(t *testing.T) {
	s := sampleSecret()
	ms := newMockServer(t, []route{secretsPostRoute(s)})

	res := runCLI(t, ms, "secrets", "add",
		"--name", "github-main",
		"--env-var", "GITHUB_TOKEN",
		"--secret", "ghp_abc123",
	)
	require.Equal(t, 0, res.ExitCode, res.Stderr)

	bodies := ms.RequestBodies("POST", "/secrets")
	require.Len(t, bodies, 1)
	_, hasHosts := bodies[0]["hosts"]
	assert.False(t, hasHosts, "hosts should not be sent when --host is not specified")
}

func TestSecretsAdd_MissingName(t *testing.T) {
	ms := newMockServer(t, nil)

	res := runCLI(t, ms, "secrets", "add",
		"--env-var", "TOKEN",
		"--secret", "x",
	)
	assert.NotEqual(t, 0, res.ExitCode)
	assert.Contains(t, res.Stderr, "--name is required")
}

func TestSecretsAdd_MissingEnvVar(t *testing.T) {
	ms := newMockServer(t, nil)

	res := runCLI(t, ms, "secrets", "add",
		"--name", "test",
		"--secret", "x",
	)
	assert.NotEqual(t, 0, res.ExitCode)
	assert.Contains(t, res.Stderr, "--env-var is required")
}

func TestSecretsAdd_WithComment(t *testing.T) {
	s := sampleSecret()
	comment := "CI publish token"
	s.Comment = &comment
	ms := newMockServer(t, []route{secretsPostRoute(s)})

	res := runCLI(t, ms, "secrets", "add",
		"--name", "npm-publish",
		"--env-var", "NPM_TOKEN",
		"--secret", "npm_abc123",
		"--comment", "CI publish token",
	)
	require.Equal(t, 0, res.ExitCode, res.Stderr)
	assert.Contains(t, res.Stdout, "CI publish token")

	bodies := ms.RequestBodies("POST", "/secrets")
	require.Len(t, bodies, 1)
	assert.Equal(t, "CI publish token", bodies[0]["comment"])
}

func TestSecretsAdd_PipedStdin(t *testing.T) {
	s := sampleSecret()
	ms := newMockServer(t, []route{secretsPostRoute(s)})

	res := runCLIWithStdin(t, ms, "ghp_from_pipe\n",
		"secrets", "add",
		"--name", "github-main",
		"--env-var", "GITHUB_TOKEN",
	)
	require.Equal(t, 0, res.ExitCode, res.Stderr)

	bodies := ms.RequestBodies("POST", "/secrets")
	require.Len(t, bodies, 1)
	assert.Equal(t, "ghp_from_pipe", bodies[0]["secret"])
}

// --- secrets list ---

func TestSecretsList(t *testing.T) {
	s := sampleSecret()
	ms := newMockServer(t, []route{secretsListRoute(s)})

	res := runCLI(t, ms, "secrets", "list")
	require.Equal(t, 0, res.ExitCode, res.Stderr)
	assert.Contains(t, res.Stdout, "github-main")
	assert.Contains(t, res.Stdout, "GITHUB_TOKEN")
	assert.Contains(t, res.Stdout, "IRONSH_PROXY_github-main")
	// Table headers
	assert.Contains(t, res.Stdout, "NAME")
	assert.Contains(t, res.Stdout, "ENV VAR")
	assert.Contains(t, res.Stdout, "HOSTS")
	assert.Contains(t, res.Stdout, "PROXY VALUE")
}

func TestSecretsList_Empty(t *testing.T) {
	ms := newMockServer(t, []route{secretsListRoute()})

	res := runCLI(t, ms, "secrets", "list")
	require.Equal(t, 0, res.ExitCode, res.Stderr)
	assert.Contains(t, res.Stdout, "No secrets found")
}

// --- secrets show ---

func TestSecretsShow_ByName(t *testing.T) {
	s := sampleSecret()
	ms := newMockServer(t, []route{
		secretsListRoute(s),
		secretsGetRoute("sec_m4xk9wp2", s),
	})

	res := runCLI(t, ms, "secrets", "show", "github-main")
	require.Equal(t, 0, res.ExitCode, res.Stderr)
	assert.Contains(t, res.Stdout, "github-main")
	assert.Contains(t, res.Stdout, "GITHUB_TOKEN")
	assert.Contains(t, res.Stdout, "IRONSH_PROXY_github-main")
	assert.NotContains(t, res.Stdout, "ghp_") // never display secret value
}

func TestSecretsShow_ByID(t *testing.T) {
	s := sampleSecret()
	ms := newMockServer(t, []route{
		secretsGetRoute("sec_m4xk9wp2", s),
	})

	res := runCLI(t, ms, "secrets", "show", "sec_m4xk9wp2")
	require.Equal(t, 0, res.ExitCode, res.Stderr)
	assert.Contains(t, res.Stdout, "sec_m4xk9wp2")
	assert.Contains(t, res.Stdout, "GITHUB_TOKEN")
	// Should not hit list endpoint for ID lookups
	assert.False(t, ms.HasRequest("GET", "/secrets"))
}

func TestSecretsShow_DisplaysHosts(t *testing.T) {
	s := sampleSecret()
	s.Hosts = []string{"api.github.com", "*.github.com"}
	ms := newMockServer(t, []route{
		secretsGetRoute("sec_m4xk9wp2", s),
	})

	res := runCLI(t, ms, "secrets", "show", "sec_m4xk9wp2")
	require.Equal(t, 0, res.ExitCode, res.Stderr)
	assert.Contains(t, res.Stdout, "api.github.com, *.github.com")
}

// --- secrets update ---

func TestSecretsUpdate_WithSecret(t *testing.T) {
	s := sampleSecret()
	ms := newMockServer(t, []route{
		secretsPatchRoute("sec_m4xk9wp2", s),
	})

	res := runCLI(t, ms, "secrets", "update", "sec_m4xk9wp2",
		"--secret", "ghp_newtoken789",
	)
	require.Equal(t, 0, res.ExitCode, res.Stderr)

	bodies := ms.RequestBodies("PATCH", "/secrets/sec_m4xk9wp2")
	require.Len(t, bodies, 1)
	assert.Equal(t, "ghp_newtoken789", bodies[0]["secret"])
}

func TestSecretsUpdate_WithHosts(t *testing.T) {
	s := sampleSecret()
	s.Hosts = []string{"api.github.com", "*.github.com"}
	ms := newMockServer(t, []route{
		secretsListRoute(s),
		secretsPatchRoute("sec_m4xk9wp2", s),
	})

	res := runCLI(t, ms, "secrets", "update", "github-main",
		"--host", "api.github.com",
		"--host", "*.github.com",
	)
	require.Equal(t, 0, res.ExitCode, res.Stderr)

	bodies := ms.RequestBodies("PATCH", "/secrets/sec_m4xk9wp2")
	require.Len(t, bodies, 1)
	assert.Equal(t, []interface{}{"api.github.com", "*.github.com"}, bodies[0]["hosts"])
}

func TestSecretsUpdate_EnvVar(t *testing.T) {
	s := sampleSecret()
	ms := newMockServer(t, []route{
		secretsListRoute(s),
		secretsPatchRoute("sec_m4xk9wp2", s),
	})

	res := runCLI(t, ms, "secrets", "update", "github-main",
		"--env-var", "GH_TOKEN",
	)
	require.Equal(t, 0, res.ExitCode, res.Stderr)

	bodies := ms.RequestBodies("PATCH", "/secrets/sec_m4xk9wp2")
	require.Len(t, bodies, 1)
	assert.Equal(t, "GH_TOKEN", bodies[0]["env_var"])
}

func TestSecretsUpdate_PipedStdin(t *testing.T) {
	s := sampleSecret()
	ms := newMockServer(t, []route{
		secretsPatchRoute("sec_m4xk9wp2", s),
	})

	res := runCLIWithStdin(t, ms, "new_secret_value\n",
		"secrets", "update", "sec_m4xk9wp2",
	)
	require.Equal(t, 0, res.ExitCode, res.Stderr)

	bodies := ms.RequestBodies("PATCH", "/secrets/sec_m4xk9wp2")
	require.Len(t, bodies, 1)
	assert.Equal(t, "new_secret_value", bodies[0]["secret"])
}

// --- secrets remove ---

func TestSecretsRemove_ByName(t *testing.T) {
	s := sampleSecret()
	ms := newMockServer(t, []route{
		secretsListRoute(s),
		secretsDeleteRoute("sec_m4xk9wp2"),
	})

	res := runCLI(t, ms, "secrets", "remove", "github-main")
	require.Equal(t, 0, res.ExitCode, res.Stderr)
	assert.Contains(t, res.Stdout, `"github-main" removed`)
	assert.True(t, ms.HasRequest("DELETE", "/secrets/sec_m4xk9wp2"))
}

func TestSecretsRemove_ByID(t *testing.T) {
	ms := newMockServer(t, []route{
		secretsDeleteRoute("sec_m4xk9wp2"),
	})

	res := runCLI(t, ms, "secrets", "remove", "sec_m4xk9wp2")
	require.Equal(t, 0, res.ExitCode, res.Stderr)
	assert.Contains(t, res.Stdout, `"sec_m4xk9wp2" removed`)
	assert.True(t, ms.HasRequest("DELETE", "/secrets/sec_m4xk9wp2"))
	assert.False(t, ms.HasRequest("GET", "/secrets"), "should not hit list for ID")
}

func TestSecretsRemove_Nonexistent(t *testing.T) {
	ms := newMockServer(t, []route{
		secretsListRoute(), // empty list
	})

	res := runCLI(t, ms, "secrets", "remove", "nonexistent")
	assert.NotEqual(t, 0, res.ExitCode)
	assert.Contains(t, res.Stderr, "no secret found")
}

// --- secrets show: never displays secret value ---

func TestSecretsShow_NeverDisplaysSecretValue(t *testing.T) {
	s := sampleSecret()
	data, err := json.Marshal(s)
	require.NoError(t, err)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))
	_, hasSecret := m["secret"]
	require.False(t, hasSecret, "Secret struct should not have a 'secret' field in JSON output")
}

// --- API client tests (these are pure unit tests, no binary needed) ---

func TestSecretsAPI_Create(t *testing.T) {
	s := sampleSecret()
	ms := newMockServer(t, []route{secretsPostRoute(s)})
	client := api.NewClient(ms.Server.URL, "test-key")

	req := api.CreateSecretRequest{
		Name:   "github-main",
		Secret: "ghp_abc123",
		EnvVar: "GITHUB_TOKEN",
	}

	got, err := client.SecretsCreate(req)
	require.NoError(t, err)
	assert.Equal(t, "sec_m4xk9wp2", got.ID)
	assert.Equal(t, "IRONSH_PROXY_github-main", got.ProxyValue)
	assert.Equal(t, []string{"*"}, got.Hosts)
}

func TestSecretsAPI_ResolveByName(t *testing.T) {
	ms := newMockServer(t, []route{secretsListRoute(sampleSecret())})
	client := api.NewClient(ms.Server.URL, "test-key")

	id, err := client.ResolveSecret("github-main")
	require.NoError(t, err)
	assert.Equal(t, "sec_m4xk9wp2", id)
}

func TestSecretsAPI_ResolveByID(t *testing.T) {
	ms := newMockServer(t, nil)
	client := api.NewClient(ms.Server.URL, "test-key")

	id, err := client.ResolveSecret("sec_m4xk9wp2")
	require.NoError(t, err)
	assert.Equal(t, "sec_m4xk9wp2", id)
	assert.Empty(t, ms.Requests(), "should not make any API calls for ID resolution")
}

func TestSecretsAPI_Delete(t *testing.T) {
	ms := newMockServer(t, []route{secretsDeleteRoute("sec_m4xk9wp2")})
	client := api.NewClient(ms.Server.URL, "test-key")

	err := client.SecretsDelete("sec_m4xk9wp2")
	require.NoError(t, err)
	assert.True(t, ms.HasRequest("DELETE", "/secrets/sec_m4xk9wp2"))
}

func TestSecretsAPI_List(t *testing.T) {
	ms := newMockServer(t, []route{secretsListRoute(sampleSecret())})
	client := api.NewClient(ms.Server.URL, "test-key")

	resp, err := client.SecretsList()
	require.NoError(t, err)
	require.Len(t, resp.Data, 1)
	assert.Equal(t, "github-main", resp.Data[0].Name)
	assert.Equal(t, []string{"*"}, resp.Data[0].Hosts)
}

func TestSecretsAPI_Update(t *testing.T) {
	ms := newMockServer(t, []route{secretsPatchRoute("sec_m4xk9wp2", sampleSecret())})
	client := api.NewClient(ms.Server.URL, "test-key")

	req := api.UpdateSecretRequest{
		Secret: "new-value",
		EnvVar: "NEW_VAR",
	}
	_, err := client.SecretsUpdate("sec_m4xk9wp2", req)
	require.NoError(t, err)

	bodies := ms.RequestBodies("PATCH", "/secrets/sec_m4xk9wp2")
	require.Len(t, bodies, 1)
	assert.Equal(t, "new-value", bodies[0]["secret"])
	assert.Equal(t, "NEW_VAR", bodies[0]["env_var"])
}

// --- output content tests ---

func TestSecretsList_TableColumns(t *testing.T) {
	s := sampleSecret()
	s2 := sampleSecret()
	s2.Name = "npm-token"
	s2.EnvVar = "NPM_TOKEN"
	s2.Hosts = []string{"registry.npmjs.org"}
	ms := newMockServer(t, []route{secretsListRoute(s, s2)})

	res := runCLI(t, ms, "secrets", "list")
	require.Equal(t, 0, res.ExitCode, res.Stderr)
	lines := strings.Split(res.Stdout, "\n")
	// Find the header line
	var headerLine string
	for _, l := range lines {
		if strings.Contains(l, "NAME") {
			headerLine = l
			break
		}
	}
	require.NotEmpty(t, headerLine, "should have a header line")
	assert.Contains(t, headerLine, "ENV VAR")
	assert.Contains(t, headerLine, "HOSTS")
	assert.Contains(t, headerLine, "PROXY VALUE")
	assert.Contains(t, headerLine, "CREATED")
	// Check data rows
	assert.Contains(t, res.Stdout, "npm-token")
	assert.Contains(t, res.Stdout, "registry.npmjs.org")
}

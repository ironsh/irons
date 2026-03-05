package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ironsh/irons/api"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

// testSecret returns a sample Secret for testing.
func testSecret() api.Secret {
	comment := ""
	return api.Secret{
		ID:         "sec_m4xk9wp2",
		Name:       "github-main",
		Provider:   "github",
		EnvVar:     "GITHUB_TOKEN",
		ProxyValue: "IRONCD_PROXY_github_github-main",
		Comment:    &comment,
		CreatedAt:  "2026-03-04T12:00:00Z",
		UpdatedAt:  "2026-03-04T12:00:00Z",
	}
}

func setupSecretsServer(handler http.HandlerFunc) (*httptest.Server, *api.Client) {
	server := httptest.NewServer(handler)
	client := api.NewClient(server.URL, "test-key")
	return server, client
}

func setupViper(serverURL string) {
	viper.Set("api-url", serverURL)
	viper.Set("api-key", "test-key")
}

func TestSecretsAdd_AllFlags(t *testing.T) {
	var gotBody api.CreateSecretRequest
	server, _ := setupSecretsServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/secrets" {
			body, _ := io.ReadAll(r.Body)
			json.Unmarshal(body, &gotBody)
			s := testSecret()
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]interface{}{"data": s})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()
	setupViper(server.URL)

	rootCmd.SetArgs([]string{"secrets", "add",
		"--name", "github-main",
		"--provider", "github",
		"--env-var", "GITHUB_TOKEN",
		"--secret", "ghp_abc123",
	})
	err := rootCmd.Execute()
	require.NoError(t, err)
	require.Equal(t, "github-main", gotBody.Name)
	require.Equal(t, "github", gotBody.Provider)
	require.Equal(t, "ghp_abc123", gotBody.Secret)
	require.Equal(t, "GITHUB_TOKEN", gotBody.EnvVar)
}

func TestSecretsAdd_MissingName(t *testing.T) {
	server, _ := setupSecretsServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()
	setupViper(server.URL)

	rootCmd.SetArgs([]string{"secrets", "add",
		"--name", "",
		"--provider", "github",
		"--env-var", "TOKEN",
		"--secret", "x",
	})
	err := rootCmd.Execute()
	require.ErrorContains(t, err, "--name is required")
}

func TestSecretsAdd_MissingProvider(t *testing.T) {
	server, _ := setupSecretsServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()
	setupViper(server.URL)

	rootCmd.SetArgs([]string{"secrets", "add",
		"--name", "test",
		"--provider", "",
		"--env-var", "TOKEN",
		"--secret", "x",
	})
	err := rootCmd.Execute()
	require.ErrorContains(t, err, "--provider is required")
}

func TestSecretsAdd_MissingEnvVar(t *testing.T) {
	server, _ := setupSecretsServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()
	setupViper(server.URL)

	rootCmd.SetArgs([]string{"secrets", "add",
		"--name", "test",
		"--provider", "github",
		"--env-var", "",
		"--secret", "x",
	})
	err := rootCmd.Execute()
	require.ErrorContains(t, err, "--env-var is required")
}

func TestSecretsAdd_UnsupportedProvider(t *testing.T) {
	server, _ := setupSecretsServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()
	setupViper(server.URL)

	rootCmd.SetArgs([]string{"secrets", "add",
		"--name", "test",
		"--provider", "bitbucket",
		"--env-var", "TOKEN",
		"--secret", "x",
	})
	err := rootCmd.Execute()
	require.ErrorContains(t, err, "unsupported provider")
}

func TestSecretsAdd_WithComment(t *testing.T) {
	var gotBody api.CreateSecretRequest
	server, _ := setupSecretsServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/secrets" {
			body, _ := io.ReadAll(r.Body)
			json.Unmarshal(body, &gotBody)
			s := testSecret()
			comment := "main token"
			s.Comment = &comment
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]interface{}{"data": s})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()
	setupViper(server.URL)

	rootCmd.SetArgs([]string{"secrets", "add",
		"--name", "github-main",
		"--provider", "github",
		"--env-var", "GITHUB_TOKEN",
		"--secret", "ghp_abc123",
		"--comment", "main token",
	})
	err := rootCmd.Execute()
	require.NoError(t, err)
	require.Equal(t, "main token", gotBody.Comment)
}

func TestSecretsList(t *testing.T) {
	server, _ := setupSecretsServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Path == "/secrets" {
			resp := api.ListSecretsResponse{
				Data: []api.Secret{testSecret()},
			}
			json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()
	setupViper(server.URL)

	rootCmd.SetArgs([]string{"secrets", "list"})
	err := rootCmd.Execute()
	require.NoError(t, err)
}

func TestSecretsShow_ByName(t *testing.T) {
	s := testSecret()
	server, _ := setupSecretsServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Path == "/secrets" {
			resp := api.ListSecretsResponse{Data: []api.Secret{s}}
			json.NewEncoder(w).Encode(resp)
			return
		}
		if r.Method == "GET" && r.URL.Path == "/secrets/sec_m4xk9wp2" {
			json.NewEncoder(w).Encode(map[string]interface{}{"data": s})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()
	setupViper(server.URL)

	rootCmd.SetArgs([]string{"secrets", "show", "github-main"})
	err := rootCmd.Execute()
	require.NoError(t, err)
}

func TestSecretsShow_ByID(t *testing.T) {
	s := testSecret()
	server, _ := setupSecretsServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Path == "/secrets/sec_m4xk9wp2" {
			json.NewEncoder(w).Encode(map[string]interface{}{"data": s})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()
	setupViper(server.URL)

	rootCmd.SetArgs([]string{"secrets", "show", "sec_m4xk9wp2"})
	err := rootCmd.Execute()
	require.NoError(t, err)
}

func TestSecretsUpdate_WithSecret(t *testing.T) {
	s := testSecret()
	var gotBody api.UpdateSecretRequest
	server, _ := setupSecretsServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PATCH" && r.URL.Path == "/secrets/sec_m4xk9wp2" {
			body, _ := io.ReadAll(r.Body)
			json.Unmarshal(body, &gotBody)
			json.NewEncoder(w).Encode(map[string]interface{}{"data": s})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()
	setupViper(server.URL)

	rootCmd.SetArgs([]string{"secrets", "update", "sec_m4xk9wp2",
		"--secret", "ghp_newtoken789",
	})
	err := rootCmd.Execute()
	require.NoError(t, err)
	require.Equal(t, "ghp_newtoken789", gotBody.Secret)
}

func TestSecretsUpdate_EnvVar(t *testing.T) {
	s := testSecret()
	var gotBody api.UpdateSecretRequest
	server, _ := setupSecretsServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Path == "/secrets" {
			resp := api.ListSecretsResponse{Data: []api.Secret{s}}
			json.NewEncoder(w).Encode(resp)
			return
		}
		if r.Method == "PATCH" && r.URL.Path == "/secrets/sec_m4xk9wp2" {
			body, _ := io.ReadAll(r.Body)
			json.Unmarshal(body, &gotBody)
			json.NewEncoder(w).Encode(map[string]interface{}{"data": s})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()
	setupViper(server.URL)

	rootCmd.SetArgs([]string{"secrets", "update", "github-main",
		"--env-var", "GH_TOKEN",
	})
	err := rootCmd.Execute()
	require.NoError(t, err)
	require.Equal(t, "GH_TOKEN", gotBody.EnvVar)
}

func TestSecretsRemove_ByName(t *testing.T) {
	s := testSecret()
	deleted := false
	server, _ := setupSecretsServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Path == "/secrets" {
			resp := api.ListSecretsResponse{Data: []api.Secret{s}}
			json.NewEncoder(w).Encode(resp)
			return
		}
		if r.Method == "DELETE" && r.URL.Path == "/secrets/sec_m4xk9wp2" {
			deleted = true
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()
	setupViper(server.URL)

	rootCmd.SetArgs([]string{"secrets", "remove", "github-main"})
	err := rootCmd.Execute()
	require.NoError(t, err)
	require.True(t, deleted, "expected DELETE to be called")
}

func TestSecretsRemove_ByID(t *testing.T) {
	deleted := false
	server, _ := setupSecretsServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" && r.URL.Path == "/secrets/sec_m4xk9wp2" {
			deleted = true
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()
	setupViper(server.URL)

	rootCmd.SetArgs([]string{"secrets", "remove", "sec_m4xk9wp2"})
	err := rootCmd.Execute()
	require.NoError(t, err)
	require.True(t, deleted, "expected DELETE to be called")
}

func TestSecretsRemove_Nonexistent(t *testing.T) {
	server, _ := setupSecretsServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Path == "/secrets" {
			resp := api.ListSecretsResponse{Data: []api.Secret{}}
			json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()
	setupViper(server.URL)

	rootCmd.SetArgs([]string{"secrets", "remove", "nonexistent"})
	err := rootCmd.Execute()
	require.ErrorContains(t, err, "no secret found")
}

func TestSecretsShow_NeverDisplaysSecretValue(t *testing.T) {
	s := testSecret()
	data, err := json.Marshal(s)
	require.NoError(t, err)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))
	_, hasSecret := m["secret"]
	require.False(t, hasSecret, "Secret struct should not have a 'secret' field in JSON output")
}

func TestSecretsAPI_Create(t *testing.T) {
	var gotBody api.CreateSecretRequest
	server, client := setupSecretsServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/secrets" {
			body, _ := io.ReadAll(r.Body)
			json.Unmarshal(body, &gotBody)
			s := testSecret()
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]interface{}{"data": s})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()

	req := api.CreateSecretRequest{
		Name:     "github-main",
		Provider: "github",
		Secret:   "ghp_abc123",
		EnvVar:   "GITHUB_TOKEN",
	}

	s, err := client.SecretsCreate(req)
	require.NoError(t, err)
	require.Equal(t, "sec_m4xk9wp2", s.ID)
	require.Equal(t, "IRONCD_PROXY_github_github-main", s.ProxyValue)
}

func TestSecretsAPI_ResolveByName(t *testing.T) {
	server, client := setupSecretsServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Path == "/secrets" {
			resp := api.ListSecretsResponse{Data: []api.Secret{testSecret()}}
			json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()

	id, err := client.ResolveSecret("github-main")
	require.NoError(t, err)
	require.Equal(t, "sec_m4xk9wp2", id)
}

func TestSecretsAPI_ResolveByID(t *testing.T) {
	server, client := setupSecretsServer(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not make any API calls for ID resolution")
	})
	defer server.Close()

	id, err := client.ResolveSecret("sec_m4xk9wp2")
	require.NoError(t, err)
	require.Equal(t, "sec_m4xk9wp2", id)
}

func TestSecretsAPI_Delete(t *testing.T) {
	deleted := false
	server, client := setupSecretsServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" && r.URL.Path == "/secrets/sec_m4xk9wp2" {
			deleted = true
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()

	err := client.SecretsDelete("sec_m4xk9wp2")
	require.NoError(t, err)
	require.True(t, deleted, "expected DELETE to be called")
}

func TestSecretsAPI_List(t *testing.T) {
	server, client := setupSecretsServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Path == "/secrets" {
			resp := api.ListSecretsResponse{
				Data:    []api.Secret{testSecret()},
				HasMore: false,
			}
			json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()

	resp, err := client.SecretsList()
	require.NoError(t, err)
	require.Len(t, resp.Data, 1)
	require.Equal(t, "github-main", resp.Data[0].Name)
}

func TestSecretsAPI_Update(t *testing.T) {
	var gotBody api.UpdateSecretRequest
	server, client := setupSecretsServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PATCH" && r.URL.Path == "/secrets/sec_m4xk9wp2" {
			body, _ := io.ReadAll(r.Body)
			json.Unmarshal(body, &gotBody)
			s := testSecret()
			json.NewEncoder(w).Encode(map[string]interface{}{"data": s})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()

	req := api.UpdateSecretRequest{
		Secret: "new-value",
		EnvVar: "NEW_VAR",
	}
	_, err := client.SecretsUpdate("sec_m4xk9wp2", req)
	require.NoError(t, err)
	require.Equal(t, "new-value", gotBody.Secret)
	require.Equal(t, "NEW_VAR", gotBody.EnvVar)
}

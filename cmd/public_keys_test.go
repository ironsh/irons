package cmd

import (
	"net/http"
	"strings"
	"testing"

	"github.com/ironsh/irons/api"
	"github.com/stretchr/testify/require"
)

func samplePublicKey() api.PublicKey {
	return api.PublicKey{
		ID:          "pub_x9f2km4p",
		Name:        "laptop",
		Fingerprint: "SHA256:abcdef1234567890",
		PublicKey:   "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI test@laptop",
		CreatedAt:   "2026-03-04T12:00:00Z",
	}
}

func publicKeysPostRoute(key api.PublicKey) route {
	return route{"POST", "/public_keys", func(w http.ResponseWriter, r *http.Request, body []byte) {
		jsonResponse(w, http.StatusCreated, wrapData(key))
	}}
}

func publicKeysListRoute(keys ...api.PublicKey) route {
	return route{"GET", "/public_keys", func(w http.ResponseWriter, r *http.Request, body []byte) {
		jsonResponse(w, http.StatusOK, api.ListPublicKeysResponse{Data: keys})
	}}
}

func publicKeysDeleteRoute(id string) route {
	return route{"DELETE", "/public_keys/" + id, func(w http.ResponseWriter, r *http.Request, body []byte) {
		w.WriteHeader(http.StatusNoContent)
	}}
}

// --- public-keys add ---

func TestPublicKeysAdd_AllFlags(t *testing.T) {
	k := samplePublicKey()
	ms := newMockServer(t, []route{publicKeysPostRoute(k)})

	res := runCLI(t, ms, "public-keys", "add",
		"--name", "laptop",
		"--public-key", "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI test@laptop",
	)
	require.Equal(t, 0, res.ExitCode, res.Stderr)
	require.Contains(t, res.Stdout, "pub_x9f2km4p")
	require.Contains(t, res.Stdout, "laptop")
	require.Contains(t, res.Stdout, "SHA256:abcdef1234567890")

	bodies := ms.RequestBodies("POST", "/public_keys")
	require.Len(t, bodies, 1)
	require.Equal(t, "laptop", bodies[0]["name"])
	require.Equal(t, "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI test@laptop", bodies[0]["public_key"])
}

func TestPublicKeysAdd_MissingName(t *testing.T) {
	ms := newMockServer(t, nil)

	res := runCLI(t, ms, "public-keys", "add",
		"--public-key", "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI test@laptop",
	)
	require.NotEqual(t, 0, res.ExitCode)
	require.Contains(t, res.Stderr, "--name is required")
}

func TestPublicKeysAdd_PipedStdin(t *testing.T) {
	k := samplePublicKey()
	ms := newMockServer(t, []route{publicKeysPostRoute(k)})

	res := runCLIWithStdin(t, ms, "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI test@laptop\n",
		"public-keys", "add",
		"--name", "laptop",
	)
	require.Equal(t, 0, res.ExitCode, res.Stderr)

	bodies := ms.RequestBodies("POST", "/public_keys")
	require.Len(t, bodies, 1)
	require.Equal(t, "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI test@laptop", bodies[0]["public_key"])
}

// --- public-keys list ---

func TestPublicKeysList(t *testing.T) {
	k := samplePublicKey()
	ms := newMockServer(t, []route{publicKeysListRoute(k)})

	res := runCLI(t, ms, "public-keys", "list")
	require.Equal(t, 0, res.ExitCode, res.Stderr)
	require.Contains(t, res.Stdout, "laptop")
	require.Contains(t, res.Stdout, "SHA256:abcdef1234567890")
	// Table headers
	require.Contains(t, res.Stdout, "NAME")
	require.Contains(t, res.Stdout, "FINGERPRINT")
	require.Contains(t, res.Stdout, "CREATED")
}

func TestPublicKeysList_Empty(t *testing.T) {
	ms := newMockServer(t, []route{publicKeysListRoute()})

	res := runCLI(t, ms, "public-keys", "list")
	require.Equal(t, 0, res.ExitCode, res.Stderr)
	require.Contains(t, res.Stdout, "No public keys found")
}

func TestPublicKeysList_Multiple(t *testing.T) {
	k1 := samplePublicKey()
	k2 := samplePublicKey()
	k2.ID = "pub_abc123"
	k2.Name = "desktop"
	k2.Fingerprint = "SHA256:zyxwvu0987654321"
	ms := newMockServer(t, []route{publicKeysListRoute(k1, k2)})

	res := runCLI(t, ms, "public-keys", "list")
	require.Equal(t, 0, res.ExitCode, res.Stderr)
	require.Contains(t, res.Stdout, "laptop")
	require.Contains(t, res.Stdout, "desktop")
}

// --- public-keys remove ---

func TestPublicKeysRemove_ByName(t *testing.T) {
	k := samplePublicKey()
	ms := newMockServer(t, []route{
		publicKeysListRoute(k),
		publicKeysDeleteRoute("pub_x9f2km4p"),
	})

	res := runCLI(t, ms, "public-keys", "remove", "laptop")
	require.Equal(t, 0, res.ExitCode, res.Stderr)
	require.Contains(t, res.Stdout, `"laptop" removed`)
	require.True(t, ms.HasRequest("DELETE", "/public_keys/pub_x9f2km4p"))
}

func TestPublicKeysRemove_ByID(t *testing.T) {
	ms := newMockServer(t, []route{
		publicKeysDeleteRoute("pub_x9f2km4p"),
	})

	res := runCLI(t, ms, "public-keys", "remove", "pub_x9f2km4p")
	require.Equal(t, 0, res.ExitCode, res.Stderr)
	require.Contains(t, res.Stdout, `"pub_x9f2km4p" removed`)
	require.True(t, ms.HasRequest("DELETE", "/public_keys/pub_x9f2km4p"))
	require.False(t, ms.HasRequest("GET", "/public_keys"), "should not hit list for ID")
}

func TestPublicKeysRemove_Nonexistent(t *testing.T) {
	ms := newMockServer(t, []route{
		publicKeysListRoute(), // empty list
	})

	res := runCLI(t, ms, "public-keys", "remove", "nonexistent")
	require.NotEqual(t, 0, res.ExitCode)
	require.Contains(t, res.Stderr, "no public key found")
}

// --- API client tests ---

func TestPublicKeysAPI_Create(t *testing.T) {
	k := samplePublicKey()
	ms := newMockServer(t, []route{publicKeysPostRoute(k)})
	client := api.NewClient(ms.Server.URL, "test-key")

	req := api.CreatePublicKeyRequest{
		Name:      "laptop",
		PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI test@laptop",
	}

	got, err := client.PublicKeysCreate(req)
	require.NoError(t, err)
	require.Equal(t, "pub_x9f2km4p", got.ID)
	require.Equal(t, "SHA256:abcdef1234567890", got.Fingerprint)
}

func TestPublicKeysAPI_List(t *testing.T) {
	ms := newMockServer(t, []route{publicKeysListRoute(samplePublicKey())})
	client := api.NewClient(ms.Server.URL, "test-key")

	resp, err := client.PublicKeysList()
	require.NoError(t, err)
	require.Len(t, resp.Data, 1)
	require.Equal(t, "laptop", resp.Data[0].Name)
}

func TestPublicKeysAPI_Delete(t *testing.T) {
	ms := newMockServer(t, []route{publicKeysDeleteRoute("pub_x9f2km4p")})
	client := api.NewClient(ms.Server.URL, "test-key")

	err := client.PublicKeysDelete("pub_x9f2km4p")
	require.NoError(t, err)
	require.True(t, ms.HasRequest("DELETE", "/public_keys/pub_x9f2km4p"))
}

func TestPublicKeysAPI_ResolveByName(t *testing.T) {
	ms := newMockServer(t, []route{publicKeysListRoute(samplePublicKey())})
	client := api.NewClient(ms.Server.URL, "test-key")

	id, err := client.ResolvePublicKey("laptop")
	require.NoError(t, err)
	require.Equal(t, "pub_x9f2km4p", id)
}

func TestPublicKeysAPI_ResolveByID(t *testing.T) {
	ms := newMockServer(t, nil)
	client := api.NewClient(ms.Server.URL, "test-key")

	id, err := client.ResolvePublicKey("pub_x9f2km4p")
	require.NoError(t, err)
	require.Equal(t, "pub_x9f2km4p", id)
	require.Empty(t, ms.Requests(), "should not make any API calls for ID resolution")
}

// --- output content tests ---

func TestPublicKeysList_TableColumns(t *testing.T) {
	k1 := samplePublicKey()
	k2 := samplePublicKey()
	k2.Name = "desktop"
	k2.Fingerprint = "SHA256:zyxwvu0987654321"
	ms := newMockServer(t, []route{publicKeysListRoute(k1, k2)})

	res := runCLI(t, ms, "public-keys", "list")
	require.Equal(t, 0, res.ExitCode, res.Stderr)
	lines := strings.Split(res.Stdout, "\n")
	var headerLine string
	for _, l := range lines {
		if strings.Contains(l, "NAME") {
			headerLine = l
			break
		}
	}
	require.NotEmpty(t, headerLine, "should have a header line")
	require.Contains(t, headerLine, "FINGERPRINT")
	require.Contains(t, headerLine, "CREATED")
	// Check data rows
	require.Contains(t, res.Stdout, "desktop")
	require.Contains(t, res.Stdout, "SHA256:zyxwvu0987654321")
}

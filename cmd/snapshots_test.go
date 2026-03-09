package cmd

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/ironsh/irons/api"
	"github.com/stretchr/testify/require"
)

func sampleSnapshot() map[string]interface{} {
	return map[string]interface{}{
		"id":            "snap_x9f2km4p",
		"vm_id":         "vm_k3mf9xvw2p",
		"label":         "pre-refactor",
		"status":        "pending",
		"base_image_id": "base_2026-02-28",
		"created_at":    "2026-03-01T14:00:00Z",
	}
}

func readySnapshot() map[string]interface{} {
	s := sampleSnapshot()
	s["status"] = "ready"
	return s
}

// --- route helpers ---

func snapshotCreateRoute(vmID string, snap map[string]interface{}) route {
	return route{
		Method: "POST",
		Path:   "/vms/" + vmID + "/snapshots",
		Handler: func(w http.ResponseWriter, r *http.Request, body []byte) {
			jsonResponse(w, http.StatusCreated, wrapData(snap))
		},
	}
}

func snapshotGetRoute(id string, snap map[string]interface{}) route {
	return route{
		Method: "GET",
		Path:   "/snapshots/" + id,
		Handler: func(w http.ResponseWriter, r *http.Request, body []byte) {
			jsonResponse(w, http.StatusOK, wrapData(snap))
		},
	}
}

func snapshotListAllRoute(snaps []map[string]interface{}) route {
	return route{
		Method: "GET",
		Path:   "/snapshots",
		Handler: func(w http.ResponseWriter, r *http.Request, body []byte) {
			jsonResponse(w, http.StatusOK, map[string]interface{}{
				"data":     snaps,
				"has_more": false,
			})
		},
	}
}

func snapshotListByVMRoute(vmID string, snaps []map[string]interface{}) route {
	return route{
		Method: "GET",
		Path:   "/vms/" + vmID + "/snapshots",
		Handler: func(w http.ResponseWriter, r *http.Request, body []byte) {
			jsonResponse(w, http.StatusOK, map[string]interface{}{
				"data":     snaps,
				"has_more": false,
			})
		},
	}
}

func snapshotDeleteRoute(id string) route {
	return route{
		Method: "DELETE",
		Path:   "/snapshots/" + id,
		Handler: func(w http.ResponseWriter, r *http.Request, body []byte) {
			w.WriteHeader(http.StatusNoContent)
		},
	}
}

// --- CLI tests ---

func TestSnapshotsCreate_Basic(t *testing.T) {
	snap := sampleSnapshot()
	ms := newMockServer(t, []route{
		snapshotCreateRoute("vm_k3mf9xvw2p", snap),
	})

	res := runCLI(t, ms, "snapshots", "create", "vm_k3mf9xvw2p", "--label", "pre-refactor")
	require.Equal(t, 0, res.ExitCode, res.Stderr)
	require.Contains(t, res.Stdout, "snap_x9f2km4p")
	require.Contains(t, res.Stdout, "pending")
	require.Contains(t, res.Stdout, "irons snapshots get snap_x9f2km4p")
}

func TestSnapshotsCreate_NoLabel(t *testing.T) {
	snap := sampleSnapshot()
	snap["label"] = ""
	ms := newMockServer(t, []route{
		snapshotCreateRoute("vm_k3mf9xvw2p", snap),
	})

	res := runCLI(t, ms, "snapshots", "create", "vm_k3mf9xvw2p")
	require.Equal(t, 0, res.ExitCode, res.Stderr)
	require.Contains(t, res.Stdout, "snap_x9f2km4p")
	require.Contains(t, res.Stdout, "(unlabeled)")
}

func TestSnapshotsCreate_ResolvesVMName(t *testing.T) {
	snap := sampleSnapshot()
	ms := newMockServer(t, []route{
		// VM name resolution
		{
			Method: "GET",
			Path:   "/vms",
			Handler: func(w http.ResponseWriter, r *http.Request, body []byte) {
				require.Equal(t, "my-dev-env", r.URL.Query().Get("name"))
				jsonResponse(w, http.StatusOK, map[string]interface{}{
					"data": []map[string]interface{}{
						{"id": "vm_k3mf9xvw2p", "name": "my-dev-env", "status": "running"},
					},
					"has_more": false,
				})
			},
		},
		snapshotCreateRoute("vm_k3mf9xvw2p", snap),
	})

	res := runCLI(t, ms, "snapshots", "create", "my-dev-env")
	require.Equal(t, 0, res.ExitCode, res.Stderr)
	require.Contains(t, res.Stdout, "snap_x9f2km4p")
	require.True(t, ms.HasRequest("POST", "/vms/vm_k3mf9xvw2p/snapshots"))
}

func TestSnapshotsCreate_JSON(t *testing.T) {
	snap := sampleSnapshot()
	ms := newMockServer(t, []route{
		snapshotCreateRoute("vm_k3mf9xvw2p", snap),
	})

	res := runCLI(t, ms, "snapshots", "create", "vm_k3mf9xvw2p", "--json")
	require.Equal(t, 0, res.ExitCode, res.Stderr)

	var parsed map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(res.Stdout), &parsed))
	require.Equal(t, "snap_x9f2km4p", parsed["id"])
}

func TestSnapshotsCreate_RequestBody(t *testing.T) {
	snap := sampleSnapshot()
	ms := newMockServer(t, []route{
		snapshotCreateRoute("vm_k3mf9xvw2p", snap),
	})

	runCLI(t, ms, "snapshots", "create", "vm_k3mf9xvw2p", "--label", "pre-refactor")

	bodies := ms.RequestBodies("POST", "/vms/vm_k3mf9xvw2p/snapshots")
	require.Len(t, bodies, 1)
	require.Equal(t, "pre-refactor", bodies[0]["label"])
}

func TestSnapshotsList_All(t *testing.T) {
	snaps := []map[string]interface{}{readySnapshot()}
	ms := newMockServer(t, []route{snapshotListAllRoute(snaps)})

	res := runCLI(t, ms, "snapshots", "list")
	require.Equal(t, 0, res.ExitCode, res.Stderr)
	require.Contains(t, res.Stdout, "snap_x9f2km4p")
	require.Contains(t, res.Stdout, "pre-refactor")
	require.Contains(t, res.Stdout, "ready")
	// VM column should be present when listing all.
	require.Contains(t, res.Stdout, "vm_k3mf9xvw2p")
}

func TestSnapshotsList_ByVM(t *testing.T) {
	snaps := []map[string]interface{}{readySnapshot()}
	ms := newMockServer(t, []route{
		snapshotListByVMRoute("vm_k3mf9xvw2p", snaps),
	})

	res := runCLI(t, ms, "snapshots", "list", "vm_k3mf9xvw2p")
	require.Equal(t, 0, res.ExitCode, res.Stderr)
	require.Contains(t, res.Stdout, "snap_x9f2km4p")
	require.True(t, ms.HasRequest("GET", "/vms/vm_k3mf9xvw2p/snapshots"))
}

func TestSnapshotsList_Empty(t *testing.T) {
	ms := newMockServer(t, []route{snapshotListAllRoute(nil)})

	res := runCLI(t, ms, "snapshots", "list")
	require.Equal(t, 0, res.ExitCode, res.Stderr)
	require.Contains(t, res.Stdout, "No snapshots found.")
}

func TestSnapshotsList_JSON(t *testing.T) {
	snaps := []map[string]interface{}{readySnapshot()}
	ms := newMockServer(t, []route{snapshotListAllRoute(snaps)})

	res := runCLI(t, ms, "snapshots", "list", "--json")
	require.Equal(t, 0, res.ExitCode, res.Stderr)

	var parsed map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(res.Stdout), &parsed))
	data, ok := parsed["data"].([]interface{})
	require.True(t, ok)
	require.Len(t, data, 1)
}

func TestSnapshotsGet_ByID(t *testing.T) {
	snap := readySnapshot()
	ms := newMockServer(t, []route{snapshotGetRoute("snap_x9f2km4p", snap)})

	res := runCLI(t, ms, "snapshots", "get", "snap_x9f2km4p")
	require.Equal(t, 0, res.ExitCode, res.Stderr)
	require.Contains(t, res.Stdout, "snap_x9f2km4p")
	require.Contains(t, res.Stdout, "pre-refactor")
	require.Contains(t, res.Stdout, "vm_k3mf9xvw2p")
	require.Contains(t, res.Stdout, "ready")
	require.Contains(t, res.Stdout, "base_2026-02-28")
}

func TestSnapshotsGet_JSON(t *testing.T) {
	snap := readySnapshot()
	ms := newMockServer(t, []route{snapshotGetRoute("snap_x9f2km4p", snap)})

	res := runCLI(t, ms, "snapshots", "get", "snap_x9f2km4p", "--json")
	require.Equal(t, 0, res.ExitCode, res.Stderr)

	var parsed map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(res.Stdout), &parsed))
	require.Equal(t, "snap_x9f2km4p", parsed["id"])
	require.Equal(t, "ready", parsed["status"])
}

func TestSnapshotsDelete_Force(t *testing.T) {
	ms := newMockServer(t, []route{snapshotDeleteRoute("snap_x9f2km4p")})

	res := runCLI(t, ms, "snapshots", "delete", "snap_x9f2km4p", "--force")
	require.Equal(t, 0, res.ExitCode, res.Stderr)
	require.Contains(t, res.Stdout, "Snapshot snap_x9f2km4p deleted.")
	require.True(t, ms.HasRequest("DELETE", "/snapshots/snap_x9f2km4p"))
}

func TestSnapshotsDelete_NoForce_PipedInput(t *testing.T) {
	snap := readySnapshot()
	ms := newMockServer(t, []route{
		snapshotGetRoute("snap_x9f2km4p", snap),
		snapshotDeleteRoute("snap_x9f2km4p"),
	})

	// Piped (non-TTY) stdin defaults to "no" → aborts.
	res := runCLI(t, ms, "snapshots", "delete", "snap_x9f2km4p")
	require.Equal(t, 0, res.ExitCode, res.Stderr)
	require.Contains(t, res.Stdout, "Aborted.")
	require.False(t, ms.HasRequest("DELETE", "/snapshots/snap_x9f2km4p"))
}

// --- API client tests ---

func TestSnapshotsAPI_Create(t *testing.T) {
	snap := sampleSnapshot()
	ms := newMockServer(t, []route{snapshotCreateRoute("vm_k3mf9xvw2p", snap)})

	client := api.NewClient(ms.Server.URL, "test-key")
	got, err := client.SnapshotsCreate("vm_k3mf9xvw2p", api.CreateSnapshotRequest{Label: "pre-refactor"})
	require.NoError(t, err)
	require.Equal(t, "snap_x9f2km4p", got.ID)
	require.Equal(t, "vm_k3mf9xvw2p", got.VMID)
	require.Equal(t, "pending", got.Status)
	require.Equal(t, "pre-refactor", got.Label)
}

func TestSnapshotsAPI_Get(t *testing.T) {
	snap := readySnapshot()
	ms := newMockServer(t, []route{snapshotGetRoute("snap_x9f2km4p", snap)})

	client := api.NewClient(ms.Server.URL, "test-key")
	got, err := client.SnapshotsGet("snap_x9f2km4p")
	require.NoError(t, err)
	require.Equal(t, "snap_x9f2km4p", got.ID)
	require.Equal(t, "ready", got.Status)
	require.Equal(t, "base_2026-02-28", got.BaseImageID)
}

func TestSnapshotsAPI_ListAll(t *testing.T) {
	snaps := []map[string]interface{}{readySnapshot()}
	ms := newMockServer(t, []route{snapshotListAllRoute(snaps)})

	client := api.NewClient(ms.Server.URL, "test-key")
	got, err := client.SnapshotsList()
	require.NoError(t, err)
	require.Len(t, got.Data, 1)
	require.Equal(t, "snap_x9f2km4p", got.Data[0].ID)
}

func TestSnapshotsAPI_ListByVM(t *testing.T) {
	snaps := []map[string]interface{}{readySnapshot()}
	ms := newMockServer(t, []route{snapshotListByVMRoute("vm_k3mf9xvw2p", snaps)})

	client := api.NewClient(ms.Server.URL, "test-key")
	got, err := client.SnapshotsListByVM("vm_k3mf9xvw2p")
	require.NoError(t, err)
	require.Len(t, got.Data, 1)
}

func TestSnapshotsAPI_Delete(t *testing.T) {
	ms := newMockServer(t, []route{snapshotDeleteRoute("snap_x9f2km4p")})

	client := api.NewClient(ms.Server.URL, "test-key")
	err := client.SnapshotsDelete("snap_x9f2km4p")
	require.NoError(t, err)
}

// --- Unit tests ---

func TestSnapshotDisplayLabel(t *testing.T) {
	require.Equal(t, "pre-refactor", snapshotDisplayLabel("pre-refactor"))
	require.Equal(t, "(unlabeled)", snapshotDisplayLabel(""))
}

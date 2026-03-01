package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Client represents the API client for VM operations
type Client struct {
	BaseURL    string
	APIKey     string
	Debug      bool
	HTTPClient *http.Client
}

// NewClient creates a new API client
func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		BaseURL: baseURL,
		APIKey:  apiKey,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// NewClientDebug creates a new API client with debug logging enabled
func NewClientDebug(baseURL, apiKey string, debug bool) *Client {
	c := NewClient(baseURL, apiKey)
	c.Debug = debug
	return c
}

// CreateRequest represents the request payload for creating a VM
type CreateRequest struct {
	PublicKey string `json:"public_key"`
	Name      string `json:"name"`
}

// VM represents a VM resource returned by the API
type VM struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Status       string `json:"status"`
	StatusDetail string `json:"status_detail,omitempty"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

// ListVMsResponse represents the response from listing all VMs.
type ListVMsResponse struct {
	Data    []VM    `json:"data"`
	HasMore bool    `json:"has_more"`
	Cursor  *string `json:"cursor,omitempty"`
}

// SSHResponse represents the response from SSH endpoint
type SSHResponse struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Command  string `json:"command,omitempty"`
}

// EgressModeRequest represents the request payload for setting the egress mode
type EgressModeRequest struct {
	Mode string `json:"mode"`
}

// EgressModeResponse represents the response from getting the egress mode
type EgressModeResponse struct {
	Mode string `json:"mode"`
}

// EgressRule represents a single egress rule.
type EgressRule struct {
	ID        string `json:"id"`
	Name      string `json:"name,omitempty"`
	Host      string `json:"host,omitempty"`
	CIDR      string `json:"cidr,omitempty"`
	Comment   string `json:"comment,omitempty"`
	CreatedAt string `json:"created_at"`
}

// EgressRuleRequest represents the request payload for creating an egress rule.
type EgressRuleRequest struct {
	Name    string `json:"name,omitempty"`
	Host    string `json:"host,omitempty"`
	CIDR    string `json:"cidr,omitempty"`
	Comment string `json:"comment,omitempty"`
}

// ListEgressRulesResponse represents the paginated response from listing egress rules.
type ListEgressRulesResponse struct {
	Data    []EgressRule `json:"data"`
	HasMore bool         `json:"has_more"`
	Cursor  *string      `json:"cursor,omitempty"`
}

// EgressAuditEvent represents a single egress audit log entry.
type EgressAuditEvent struct {
	ID        string    `json:"id"`
	VMID      string    `json:"vm_id"`
	Timestamp time.Time `json:"timestamp"`
	Host      string    `json:"host"`
	CIDR      string    `json:"cidr,omitempty"`
	Protocol  string    `json:"protocol,omitempty"`
	Allowed   bool      `json:"allowed"`
	Verdict   string    `json:"verdict,omitempty"`
	Mode      string    `json:"mode,omitempty"`
}

// ListAuditEgressResponse is the paginated response for GET /audit/egress.
type ListAuditEgressResponse struct {
	Data    []EgressAuditEvent `json:"data"`
	HasMore bool               `json:"has_more"`
	Cursor  string             `json:"cursor,omitempty"`
}

// AuditEgressParams contains query parameters for the audit egress endpoint.
type AuditEgressParams struct {
	VMID    string
	Verdict string
	Since   string
	Until   string
	Limit   int
	Cursor  string
}

// DeviceCodeResponse represents the response from POST /auth/device/code
type DeviceCodeResponse struct {
	Code            string    `json:"code"`
	VerificationURI string    `json:"verification_uri"`
	ExpiresAt       time.Time `json:"expires_at"`
}

// PollResponse represents the response from GET /auth/device/poll
type PollResponse struct {
	Status string `json:"status"`
	Token  string `json:"token,omitempty"`
}

// ErrorResponse represents an error response from the API
type ErrorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// dataWrapper is used to decode singular API responses wrapped in a "data" key.
type dataWrapper[T any] struct {
	Data T `json:"data"`
}

// unwrapData decodes a singular response body that is wrapped in a top-level
// "data" key and returns the inner value.
func unwrapData[T any](body []byte) (T, error) {
	var w dataWrapper[T]
	if err := json.Unmarshal(body, &w); err != nil {
		var zero T
		return zero, err
	}
	return w.Data, nil
}

// Create creates a new VM
func (c *Client) Create(key []byte, name string) (*VM, error) {
	req := CreateRequest{
		PublicKey: string(key),
		Name:      name,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	body, err := c.makeRequest("POST", "/vms", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create VM: %w", err)
	}

	vm, err := unwrapData[VM](body)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &vm, nil
}

// GetVM retrieves a VM by ID
func (c *Client) GetVM(id string) (*VM, error) {
	path := fmt.Sprintf("/vms/%s", id)

	body, err := c.makeRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get VM: %w", err)
	}

	vm, err := unwrapData[VM](body)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &vm, nil
}

// ListVMs lists all VMs
func (c *Client) ListVMs() (*ListVMsResponse, error) {
	body, err := c.makeRequest("GET", "/vms", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list VMs: %w", err)
	}

	var listResp ListVMsResponse
	if err := json.Unmarshal(body, &listResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &listResp, nil
}

// ListVMsByName lists VMs filtered by name.
func (c *Client) ListVMsByName(name string) (*ListVMsResponse, error) {
	q := url.Values{}
	q.Set("name", name)
	path := "/vms?" + q.Encode()

	body, err := c.makeRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list VMs by name: %w", err)
	}

	var listResp ListVMsResponse
	if err := json.Unmarshal(body, &listResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &listResp, nil
}

// ResolveVM resolves a VM identifier to a VM ID. If idOrName already looks
// like a VM ID (starts with "vm_") it is returned as-is. Otherwise the value
// is treated as a name: the list VMs endpoint is queried with that name and
// the first non-destroyed VM in the result is returned.
func (c *Client) ResolveVM(idOrName string) (string, error) {
	if strings.HasPrefix(idOrName, "vm_") {
		return idOrName, nil
	}

	resp, err := c.ListVMsByName(idOrName)
	if err != nil {
		return "", fmt.Errorf("resolving VM name %q: %w", idOrName, err)
	}

	for _, vm := range resp.Data {
		if vm.Status != "destroyed" {
			return vm.ID, nil
		}
	}

	return "", fmt.Errorf("no active VM found with name %q", idOrName)
}

// SSH retrieves SSH connection information for a VM
func (c *Client) SSH(id string) (*SSHResponse, error) {
	path := fmt.Sprintf("/vms/%s/ssh", id)

	body, err := c.makeRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get SSH info: %w", err)
	}

	sshResp, err := unwrapData[SSHResponse](body)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &sshResp, nil
}

// Destroy destroys a VM
func (c *Client) Destroy(id string) error {
	path := fmt.Sprintf("/vms/%s", id)

	_, err := c.makeRequest("DELETE", path, nil)
	if err != nil {
		return fmt.Errorf("failed to destroy VM: %w", err)
	}

	return nil
}

// Start starts a VM
func (c *Client) Start(id string) (*VM, error) {
	path := fmt.Sprintf("/vms/%s/start", id)

	body, err := c.makeRequest("POST", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to start VM: %w", err)
	}

	vm, err := unwrapData[VM](body)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &vm, nil
}

// Stop stops a VM
func (c *Client) Stop(id string) (*VM, error) {
	path := fmt.Sprintf("/vms/%s/stop", id)

	body, err := c.makeRequest("POST", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to stop VM: %w", err)
	}

	vm, err := unwrapData[VM](body)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &vm, nil
}

// EgressGetPolicy gets the current egress policy for the account
func (c *Client) EgressGetPolicy() (*EgressModeResponse, error) {
	body, err := c.makeRequest("GET", "/egress/policy", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get egress policy: %w", err)
	}

	modeResp, err := unwrapData[EgressModeResponse](body)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &modeResp, nil
}

// EgressSetPolicy sets the egress policy for the account
func (c *Client) EgressSetPolicy(mode string) error {
	req := EgressModeRequest{Mode: mode}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	_, err = c.makeRequest("PUT", "/egress/policy", bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("failed to set egress policy: %w", err)
	}

	return nil
}

// EgressListRules lists all egress rules for the account
func (c *Client) EgressListRules() (*ListEgressRulesResponse, error) {
	body, err := c.makeRequest("GET", "/egress/rules", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list egress rules: %w", err)
	}

	var listResp ListEgressRulesResponse
	if err := json.Unmarshal(body, &listResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &listResp, nil
}

// EgressCreateRule creates a new egress rule
func (c *Client) EgressCreateRule(req EgressRuleRequest) (*EgressRule, error) {
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	body, err := c.makeRequest("POST", "/egress/rules", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create egress rule: %w", err)
	}

	rule, err := unwrapData[EgressRule](body)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &rule, nil
}

// EgressDeleteRule deletes an egress rule by ID
func (c *Client) EgressDeleteRule(id string) error {
	path := fmt.Sprintf("/egress/rules/%s", id)

	_, err := c.makeRequest("DELETE", path, nil)
	if err != nil {
		return fmt.Errorf("failed to delete egress rule: %w", err)
	}

	return nil
}

// VMEgressGetPolicy gets the egress policy for a specific VM
func (c *Client) VMEgressGetPolicy(vmID string) (*EgressModeResponse, error) {
	path := fmt.Sprintf("/vms/%s/egress/policy", vmID)

	body, err := c.makeRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get VM egress policy: %w", err)
	}

	modeResp, err := unwrapData[EgressModeResponse](body)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &modeResp, nil
}

// VMEgressSetPolicy sets the egress policy for a specific VM
func (c *Client) VMEgressSetPolicy(vmID, mode string) error {
	req := EgressModeRequest{Mode: mode}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	path := fmt.Sprintf("/vms/%s/egress/policy", vmID)
	_, err = c.makeRequest("PUT", path, bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("failed to set VM egress policy: %w", err)
	}

	return nil
}

// AuditEgress fetches egress audit events with the given query parameters.
func (c *Client) AuditEgress(params AuditEgressParams) (*ListAuditEgressResponse, error) {
	q := url.Values{}
	if params.VMID != "" {
		q.Set("vm_id", params.VMID)
	}
	if params.Verdict != "" {
		q.Set("verdict", params.Verdict)
	}
	if params.Since != "" {
		q.Set("since", params.Since)
	}
	if params.Until != "" {
		q.Set("until", params.Until)
	}
	if params.Limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", params.Limit))
	}
	if params.Cursor != "" {
		q.Set("cursor", params.Cursor)
	}

	path := "/audit/egress"
	if encoded := q.Encode(); encoded != "" {
		path += "?" + encoded
	}

	body, err := c.makeRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch egress audit log: %w", err)
	}

	var auditResp ListAuditEgressResponse
	if err := json.Unmarshal(body, &auditResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &auditResp, nil
}

// DeviceCode requests a new device code to begin the device authorization flow.
func (c *Client) DeviceCode() (*DeviceCodeResponse, error) {
	body, err := c.makeRequest("POST", "/auth/device/code", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to request device code: %w", err)
	}

	resp, err := unwrapData[DeviceCodeResponse](body)
	if err != nil {
		return nil, fmt.Errorf("failed to decode device code response: %w", err)
	}

	return &resp, nil
}

// PollDevice polls the device authorization endpoint for the given code.
func (c *Client) PollDevice(code string) (*PollResponse, error) {
	path := fmt.Sprintf("/auth/device/poll?code=%s", code)

	body, err := c.makeRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to poll device auth: %w", err)
	}

	resp, err := unwrapData[PollResponse](body)
	if err != nil {
		return nil, fmt.Errorf("failed to decode poll response: %w", err)
	}

	return &resp, nil
}

// makeRequest makes an HTTP request with common headers and error handling
func (c *Client) makeRequest(method, path string, body io.Reader) ([]byte, error) {
	reqURL := c.BaseURL + path

	// Buffer the body so we can both log it and send it.
	var reqBytes []byte
	if body != nil && c.Debug {
		var err error
		reqBytes, err = io.ReadAll(body)
		if err != nil {
			return nil, fmt.Errorf("failed to read request body: %w", err)
		}
		body = bytes.NewReader(reqBytes)
	}

	if c.Debug {
		fmt.Fprintf(os.Stderr, ">>> %s %s\n", method, path)
		if len(reqBytes) > 0 {
			fmt.Fprintf(os.Stderr, "%s\n", reqBytes)
		}
	}

	req, err := http.NewRequest(method, reqURL, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	// Check for no-content response
	if resp.StatusCode == http.StatusNoContent {
		if c.Debug {
			fmt.Fprintf(os.Stderr, "<<< %d (no content)\n", resp.StatusCode)
		}
		return nil, nil
	}

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if c.Debug {
		fmt.Fprintf(os.Stderr, "<<< %d\n%s\n", resp.StatusCode, respBody)
	}

	// Check for HTTP errors
	if resp.StatusCode >= 400 {
		// Try to parse as JSON error response
		var errResp ErrorResponse
		if err := json.Unmarshal(respBody, &errResp); err == nil && errResp.Error.Message != "" {
			return nil, fmt.Errorf("API error: %s", errResp.Error.Message)
		}

		// Fallback to raw body if JSON parsing fails
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

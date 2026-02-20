package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client represents the API client for sandbox operations
type Client struct {
	BaseURL    string
	APIKey     string
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

// CreateRequest represents the request payload for creating a sandbox
type CreateRequest struct {
	PublicKey string `json:"public_key"`
	Name      string `json:"name"`
}

// CreateResponse represents the response from creating a sandbox
type CreateResponse struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

// SSHResponse represents the response from SSH endpoint
type SSHResponse struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Command  string `json:"command,omitempty"`
}

// StatusResponse represents the response from status endpoint
type StatusResponse struct {
	Name      string            `json:"name"`
	ID        string            `json:"id"`
	Status    string            `json:"status"`
	CreatedAt string            `json:"created_at"`
	UpdatedAt string            `json:"updated_at"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// DestroyResponse represents the response from destroy endpoint
type DestroyResponse struct {
	Name string `json:"name"`
}

// ListSandboxesResponse represents the response from listing all sandboxes.
type ListSandboxesResponse struct {
	Sandboxes []StatusResponse `json:"sandboxes"`
}

// EgressAllowRequest represents the request payload for allowing egress to a domain
type EgressAllowRequest struct {
	Domain string `json:"domain"`
}

// EgressDenyRequest represents the request payload for denying egress to a domain
type EgressDenyRequest struct {
	Domain string `json:"domain"`
}

// EgressModeRequest represents the request payload for setting the egress mode
type EgressModeRequest struct {
	Mode string `json:"mode"`
}

// EgressModeResponse represents the response from getting the egress mode
type EgressModeResponse struct {
	Mode string `json:"mode"`
}

// EgressListResponse represents the response from listing egress rules
type EgressListResponse struct {
	AllowedDomains []string `json:"allowed_domains"`
	DeniedDomains  []string `json:"denied_domains"`
}

// EgressAuditEvent represents a single egress audit log entry.
type EgressAuditEvent struct {
	Type        string    `json:"type"`
	Timestamp   time.Time `json:"timestamp"`
	SandboxName string    `json:"sandbox_name"`
	Host        string    `json:"host"`
	Allowed     bool      `json:"allowed"`
	Mode        string    `json:"mode,omitempty"`
}

// EgressAuditResponse is the paginated response for GET /sandboxes/{name}/audit/egress.
type EgressAuditResponse struct {
	Events    []EgressAuditEvent `json:"events"`
	PageToken int64              `json:"page_token,omitempty"`
}

// ErrorResponse represents an error response from the API
type ErrorResponse struct {
	Error string `json:"error"`
}

// Create creates a new sandbox
func (c *Client) Create(key []byte, name string) (*CreateResponse, error) {
	req := CreateRequest{
		PublicKey: string(key),
		Name:      name,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	body, err := c.makeRequest("POST", "/sandboxes", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create sandbox: %w", err)
	}

	var createResp CreateResponse
	if err := json.Unmarshal(body, &createResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &createResp, nil
}

// SSH retrieves SSH connection information for a sandbox
func (c *Client) SSH(name string) (*SSHResponse, error) {
	url := fmt.Sprintf("/sandboxes/%s/ssh", name)

	body, err := c.makeRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get SSH info: %w", err)
	}

	var sshResp SSHResponse
	if err := json.Unmarshal(body, &sshResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &sshResp, nil
}

// Destroy destroys a sandbox
func (c *Client) Destroy(name string) error {
	url := fmt.Sprintf("/sandboxes/%s", name)

	_, err := c.makeRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to destroy sandbox: %w", err)
	}

	return nil
}

// List lists all sandboxes
func (c *Client) List() (*ListSandboxesResponse, error) {
	body, err := c.makeRequest("GET", "/sandboxes", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list sandboxes: %w", err)
	}

	var listResp ListSandboxesResponse
	if err := json.Unmarshal(body, &listResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &listResp, nil
}

// Status gets the status of a sandbox
func (c *Client) Status(name string) (*StatusResponse, error) {
	url := fmt.Sprintf("/sandboxes/%s", name)

	body, err := c.makeRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get status: %w", err)
	}

	var statusResp StatusResponse
	if err := json.Unmarshal(body, &statusResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &statusResp, nil
}

// EgressAllow allows egress traffic to a domain for a sandbox
func (c *Client) EgressAllow(domain string) error {
	url := "/egress/allow"

	req := EgressAllowRequest{
		Domain: domain,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	_, err = c.makeRequest("POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("failed to allow egress: %w", err)
	}

	return nil
}

// EgressDeny denies egress traffic to a domain for a sandbox
func (c *Client) EgressDeny(domain string) error {
	url := "/egress/deny"

	req := EgressDenyRequest{
		Domain: domain,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	_, err = c.makeRequest("POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("failed to deny egress: %w", err)
	}

	return nil
}

// EgressGetMode gets the current egress mode for the account
func (c *Client) EgressGetMode() (*EgressModeResponse, error) {
	body, err := c.makeRequest("GET", "/egress/mode", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get egress mode: %w", err)
	}

	var modeResp EgressModeResponse
	if err := json.Unmarshal(body, &modeResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &modeResp, nil
}

// EgressSetMode sets the egress mode for the account
func (c *Client) EgressSetMode(mode string) error {
	req := EgressModeRequest{Mode: mode}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	_, err = c.makeRequest("PUT", "/egress/mode", bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("failed to set egress mode: %w", err)
	}

	return nil
}

// EgressList lists all egress rules for the account
func (c *Client) EgressList() (*EgressListResponse, error) {
	url := "/egress"

	body, err := c.makeRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list egress rules: %w", err)
	}

	var listResp EgressListResponse
	if err := json.Unmarshal(body, &listResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &listResp, nil
}

// Start starts a sandbox
func (c *Client) Start(name string) error {
	url := fmt.Sprintf("/sandboxes/%s/start", name)

	_, err := c.makeRequest("POST", url, nil)
	if err != nil {
		return fmt.Errorf("failed to start sandbox: %w", err)
	}

	return nil
}

// Stop stops a sandbox
func (c *Client) Stop(name string) error {
	url := fmt.Sprintf("/sandboxes/%s/stop", name)

	_, err := c.makeRequest("POST", url, nil)
	if err != nil {
		return fmt.Errorf("failed to stop sandbox: %w", err)
	}

	return nil
}

// AuditEgress fetches egress audit events for a sandbox. Pass pageToken=0 for
// the first request; subsequent requests should pass the PageToken from the
// previous response to receive only newly-appended events.
func (c *Client) AuditEgress(name string, pageToken int64) (*EgressAuditResponse, error) {
	path := fmt.Sprintf("/sandboxes/%s/audit/egress", name)
	if pageToken > 0 {
		path = fmt.Sprintf("%s?pageToken=%d", path, pageToken)
	}

	body, err := c.makeRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch egress audit log: %w", err)
	}

	var auditResp EgressAuditResponse
	if err := json.Unmarshal(body, &auditResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &auditResp, nil
}

// makeRequest makes an HTTP request with common headers and error handling
func (c *Client) makeRequest(method, path string, body io.Reader) ([]byte, error) {
	url := c.BaseURL + path

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	// Check for no-content response
	if resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check for HTTP errors
	if resp.StatusCode >= 400 {
		fmt.Println(string(respBody), resp.StatusCode)
		// Try to parse as JSON error response
		var errResp ErrorResponse
		if err := json.Unmarshal(respBody, &errResp); err == nil && errResp.Error != "" {
			return nil, fmt.Errorf("API error: %s", errResp.Error)
		}

		// Fallback to raw body if JSON parsing fails
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

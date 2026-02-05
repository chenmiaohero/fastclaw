package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// GatewayRequest represents a JSON-RPC style request to the Gateway
type GatewayRequest struct {
	Type   string      `json:"type"`
	ID     int64       `json:"id"`
	Method string      `json:"method"`
	Params interface{} `json:"params,omitempty"`
}

// GatewayResponse represents a JSON-RPC style response from the Gateway
type GatewayResponse struct {
	Type    string          `json:"type"`
	ID      int64           `json:"id"`
	OK      bool            `json:"ok"`
	Payload json.RawMessage `json:"payload,omitempty"`
	Error   *GatewayError   `json:"error,omitempty"`
}

// GatewayError represents an error from the Gateway
type GatewayError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// NodePairListResponse represents the response from node.pair.list
type NodePairListResponse struct {
	Requests []PendingPairRequest `json:"requests"`
}

// PendingPairRequest represents a pending pairing request
type PendingPairRequest struct {
	RequestID  string `json:"requestId"`
	DeviceID   string `json:"deviceId"`
	Role       string `json:"role"`
	Platform   string `json:"platform"`
	ClientID   string `json:"clientId"`
	ClientMode string `json:"clientMode"`
	IP         string `json:"ip"`
	Ts         int64  `json:"ts"` // timestamp in ms
}

// NodeListResponse represents the response from node.list
type NodeListResponse struct {
	Nodes []PairedNode `json:"nodes"`
}

// PairedNode represents a paired node/device
type PairedNode struct {
	DeviceID        string      `json:"deviceId"`
	Role            string      `json:"role"`
	Platform        string      `json:"platform"`
	ClientID        string      `json:"clientId"`
	ClientMode      string      `json:"clientMode"`
	DeviceFamily    string      `json:"deviceFamily,omitempty"`
	ModelIdentifier string      `json:"modelIdentifier,omitempty"`
	Paired          bool        `json:"paired"`
	Connected       bool        `json:"connected"`
	CreatedAtMs     int64       `json:"createdAtMs"`
	ApprovedAtMs    int64       `json:"approvedAtMs"`
	Tokens          []NodeToken `json:"tokens,omitempty"`
}

// NodeToken represents a token for a paired node
type NodeToken struct {
	Role        string `json:"role"`
	RevokedAtMs int64  `json:"revokedAtMs,omitempty"`
}

// GatewayClient is a client for the OpenClaw Gateway WebSocket API
type GatewayClient struct {
	conn      *websocket.Conn
	requestID int64
}

// NewGatewayClient creates a new Gateway client and connects to the specified endpoint
func NewGatewayClient(ctx context.Context, endpoint, accessToken string) (*GatewayClient, error) {
	// Build WebSocket URL - try with token in URL first
	wsURL := fmt.Sprintf("ws://%s/?token=%s", endpoint, accessToken)

	// Set up dialer with timeout
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	// Build headers with multiple auth methods for compatibility
	headers := http.Header{
		"Origin":        []string{fmt.Sprintf("http://%s", endpoint)},
		"Authorization": []string{fmt.Sprintf("Bearer %s", accessToken)},
	}

	// Connect
	conn, resp, err := dialer.DialContext(ctx, wsURL, headers)
	if err != nil {
		errMsg := fmt.Sprintf("failed to connect to gateway at %s: %v", wsURL, err)
		if resp != nil {
			errMsg += fmt.Sprintf(" (status: %d)", resp.StatusCode)
		}
		return nil, fmt.Errorf("%s", errMsg)
	}

	client := &GatewayClient{
		conn:      conn,
		requestID: 0,
	}

	// Handle authentication handshake
	if err := client.handleAuth(ctx, accessToken); err != nil {
		conn.Close()
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	return client, nil
}

// handleAuth handles the authentication challenge from Gateway
func (c *GatewayClient) handleAuth(ctx context.Context, accessToken string) error {
	// Set read deadline for auth
	c.conn.SetReadDeadline(time.Now().Add(10 * time.Second))

	// Read the challenge event
	_, message, err := c.conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("failed to read challenge: %w", err)
	}

	fmt.Printf("[Gateway] Auth: received message: %s\n", string(message))

	// Parse the message
	var event struct {
		Type    string `json:"type"`
		Event   string `json:"event"`
		Payload struct {
			Nonce string `json:"nonce"`
			Ts    int64  `json:"ts"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(message, &event); err != nil {
		return fmt.Errorf("failed to parse challenge: %w", err)
	}

	// Check if it's a connect.challenge event
	if event.Type != "event" || event.Event != "connect.challenge" {
		// Not a challenge, maybe already authenticated
		fmt.Printf("[Gateway] Auth: not a challenge event, skipping auth\n")
		return nil
	}

	// Send auth response with password token
	authReq := map[string]interface{}{
		"type":   "req",
		"id":     0,
		"method": "connect",
		"params": map[string]interface{}{
			"auth": map[string]interface{}{
				"mode":     "password",
				"password": accessToken,
			},
			"nonce": event.Payload.Nonce,
		},
	}

	authReqBytes, _ := json.Marshal(authReq)
	fmt.Printf("[Gateway] Auth: sending connect request: %s\n", string(authReqBytes))

	c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if err := c.conn.WriteJSON(authReq); err != nil {
		return fmt.Errorf("failed to send auth: %w", err)
	}

	// Read auth response
	c.conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	_, respMsg, err := c.conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("failed to read auth response: %w", err)
	}

	fmt.Printf("[Gateway] Auth: received response: %s\n", string(respMsg))

	// Parse response
	var resp struct {
		Type  string `json:"type"`
		ID    int64  `json:"id"`
		OK    bool   `json:"ok"`
		Error *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(respMsg, &resp); err != nil {
		return fmt.Errorf("failed to parse auth response: %w", err)
	}

	if resp.Type == "res" && !resp.OK {
		if resp.Error != nil {
			return fmt.Errorf("auth error [%s]: %s", resp.Error.Code, resp.Error.Message)
		}
		return fmt.Errorf("auth failed")
	}

	fmt.Printf("[Gateway] Auth: authentication successful\n")
	return nil
}

// Close closes the WebSocket connection
func (c *GatewayClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// Call sends a request to the Gateway and waits for a response
func (c *GatewayClient) Call(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	id := atomic.AddInt64(&c.requestID, 1)

	req := GatewayRequest{
		Type:   "req",
		ID:     id,
		Method: method,
		Params: params,
	}

	// Debug: log request
	reqBytes, _ := json.Marshal(req)
	fmt.Printf("[Gateway] Sending request: %s\n", string(reqBytes))

	// Set write deadline
	if deadline, ok := ctx.Deadline(); ok {
		c.conn.SetWriteDeadline(deadline)
	} else {
		c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	}

	// Send request
	if err := c.conn.WriteJSON(req); err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Set read deadline
	if deadline, ok := ctx.Deadline(); ok {
		c.conn.SetReadDeadline(deadline)
	} else {
		c.conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	}

	// Read responses until we get the one we're waiting for
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			return nil, fmt.Errorf("failed to read response: %w", err)
		}

		// Debug: log raw message
		fmt.Printf("[Gateway] Received message: %s\n", string(message))

		var resp GatewayResponse
		if err := json.Unmarshal(message, &resp); err != nil {
			// Skip non-JSON messages or events
			fmt.Printf("[Gateway] Failed to parse as response, skipping: %v\n", err)
			continue
		}

		// Skip events (type: "event")
		if resp.Type == "event" {
			fmt.Printf("[Gateway] Skipping event message\n")
			continue
		}

		// Check if this is our response
		if resp.Type == "res" && resp.ID == id {
			if !resp.OK {
				if resp.Error != nil {
					return nil, fmt.Errorf("gateway error [%s]: %s", resp.Error.Code, resp.Error.Message)
				}
				return nil, fmt.Errorf("gateway returned error, payload: %s", string(resp.Payload))
			}
			return resp.Payload, nil
		}
	}
}

// GetPendingPairRequests gets the list of pending pairing requests
func (c *GatewayClient) GetPendingPairRequests(ctx context.Context) ([]PendingPairRequest, error) {
	payload, err := c.Call(ctx, "node.pair.list", nil)
	if err != nil {
		return nil, err
	}

	// Debug: log raw payload
	fmt.Printf("[Gateway] node.pair.list raw response: %s\n", string(payload))

	// Try to parse as array first (some versions return array directly)
	var requests []PendingPairRequest
	if err := json.Unmarshal(payload, &requests); err == nil {
		return requests, nil
	}

	// Try to parse as object with requests field
	var resp NodePairListResponse
	if err := json.Unmarshal(payload, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse node.pair.list response: %w, raw: %s", err, string(payload))
	}

	return resp.Requests, nil
}

// GetPairedNodes gets the list of paired nodes/devices
func (c *GatewayClient) GetPairedNodes(ctx context.Context) ([]PairedNode, error) {
	payload, err := c.Call(ctx, "node.list", nil)
	if err != nil {
		return nil, err
	}

	// Debug: log raw payload
	fmt.Printf("[Gateway] node.list raw response: %s\n", string(payload))

	// Try to parse as array first
	var nodes []PairedNode
	if err := json.Unmarshal(payload, &nodes); err == nil {
		return nodes, nil
	}

	// Try to parse as object with nodes field
	var resp NodeListResponse
	if err := json.Unmarshal(payload, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse node.list response: %w, raw: %s", err, string(payload))
	}

	return resp.Nodes, nil
}

// ListBotDevicesViaGateway lists devices using the Gateway WebSocket API
// This is much faster than executing CLI commands via ExecInPod
func ListBotDevicesViaGateway(ctx context.Context, botID, accessToken string) (*DeviceListResult, error) {
	// Get service endpoint
	endpoint, err := GetServiceEndpoint(ctx, botID)
	if err != nil {
		return nil, fmt.Errorf("failed to get service endpoint: %w", err)
	}
	if endpoint == "" {
		return nil, fmt.Errorf("bot service not found")
	}

	// Create gateway client with timeout
	clientCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	client, err := NewGatewayClient(clientCtx, endpoint, accessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create gateway client: %w", err)
	}
	defer client.Close()

	result := &DeviceListResult{
		Pending: []PendingPairRequest{},
		Paired:  []PairedNode{},
	}

	// Get pending pair requests
	pending, err := client.GetPendingPairRequests(clientCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending pair requests: %w", err)
	}
	result.Pending = pending

	// Get paired nodes
	paired, err := client.GetPairedNodes(clientCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to get paired nodes: %w", err)
	}
	result.Paired = paired

	return result, nil
}

// DeviceListResult contains both pending and paired devices
type DeviceListResult struct {
	Pending []PendingPairRequest `json:"pending"`
	Paired  []PairedNode         `json:"paired"`
}

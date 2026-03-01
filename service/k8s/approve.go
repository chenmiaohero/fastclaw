package k8s

import (
	"context"
	"fmt"
	"time"
)

// AutoApproveAllPending approves all pending device pairing requests for a bot.
// Uses the Gateway WebSocket API directly from fastclaw, bypassing the CLI.
// This avoids the wss:// security check that OpenClaw 2.19+ enforces on CLI commands.
func AutoApproveAllPending(ctx context.Context, botID, accessToken string) error {
	// Get service endpoint
	endpoint, err := GetServiceEndpoint(ctx, botID)
	if err != nil {
		return fmt.Errorf("failed to get service endpoint: %w", err)
	}
	if endpoint == "" {
		return fmt.Errorf("bot service not found")
	}

	// Connect to gateway with timeout
	clientCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	client, err := NewGatewayClient(clientCtx, endpoint, accessToken)
	if err != nil {
		return fmt.Errorf("failed to connect to gateway: %w", err)
	}
	defer client.Close()

	// List pending pair requests
	pending, err := client.GetPendingPairRequests(clientCtx)
	if err != nil {
		return fmt.Errorf("failed to list pending devices: %w", err)
	}

	if len(pending) == 0 {
		return nil
	}

	// Approve each pending device
	for _, req := range pending {
		if err := client.ApprovePairRequest(clientCtx, req.RequestID); err != nil {
			fmt.Printf("[AutoApprove] Failed to approve %s: %v\n", req.RequestID, err)
			continue
		}
		fmt.Printf("[AutoApprove] Approved device %s (mode: %s, ip: %s)\n", req.DeviceID, req.ClientMode, req.IP)
	}

	return nil
}

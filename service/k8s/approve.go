package k8s

import (
	"context"
	"encoding/json"
	"fmt"
)

// pendingDeviceList represents the JSON output from openclaw devices list --json
type pendingDeviceList struct {
	Pending []struct {
		RequestID string `json:"requestId"`
		DeviceID  string `json:"deviceId"`
		ClientMode string `json:"clientMode"`
		IP        string `json:"ip"`
	} `json:"pending"`
}

// AutoApproveAllPending approves all pending device pairing requests for a bot.
// Uses CLI commands via ExecInPod for reliability.
func AutoApproveAllPending(ctx context.Context, botID, accessToken string) error {
	namespace := GetNamespace()

	podName, err := GetPodName(ctx, botID)
	if err != nil {
		return fmt.Errorf("failed to get pod: %w", err)
	}

	// List pending devices via CLI
	output, err := ExecInPod(ctx, namespace, podName, "openclaw",
		[]string{"node", "/app/openclaw.mjs", "devices", "list", "--json", "--token", accessToken})
	if err != nil {
		return fmt.Errorf("failed to list devices: %w", err)
	}

	var deviceList pendingDeviceList
	if err := json.Unmarshal([]byte(output), &deviceList); err != nil {
		return fmt.Errorf("failed to parse devices: %w", err)
	}

	if len(deviceList.Pending) == 0 {
		return nil
	}

	// Approve each pending device
	for _, req := range deviceList.Pending {
		_, err := ExecInPod(ctx, namespace, podName, "openclaw",
			[]string{"node", "/app/openclaw.mjs", "devices", "approve", req.RequestID, "--token", accessToken})
		if err != nil {
			fmt.Printf("[AutoApprove] Failed to approve %s: %v\n", req.RequestID, err)
			continue
		}
		fmt.Printf("[AutoApprove] Approved device %s (mode: %s, ip: %s)\n", req.DeviceID, req.ClientMode, req.IP)
	}

	return nil
}

package k8s

import (
	"context"
	"strings"
	"time"
)

// StartAutoApprove starts auto-approval of pairing requests for a bot
// This should be called after bot starts, regardless of config
func StartAutoApprove(botID string) {
	go func() {
		ctx := context.Background()
		// Wait for pod to be ready first
		podName, err := waitForPodReady(ctx, botID, 120)
		if err != nil {
			return
		}
		autoApprovePairingRequests(botID, podName)
	}()
}

// autoApprovePairingRequests periodically checks and approves pending device pairing requests
func autoApprovePairingRequests(botID, podName string) {
	namespace := GetNamespace()
	ctx := context.Background()

	// Check and approve for 5 minutes after bot starts
	for i := 0; i < 60; i++ {
		time.Sleep(5 * time.Second)

		// List pending pairing requests and approve all
		output, err := ExecInPod(ctx, namespace, podName, "openclaw",
			[]string{"node", "/app/openclaw.mjs", "devices", "list"})
		if err != nil {
			continue
		}

		// Parse output and approve pending requests
		approvePendingDevices(ctx, namespace, podName, output)
	}
}

// approvePendingDevices parses device list and approves pending ones
func approvePendingDevices(ctx context.Context, namespace, podName, output string) {
	lines := strings.Split(output, "\n")
	inPendingSection := false

	for _, line := range lines {
		if strings.Contains(line, "Pending") {
			inPendingSection = true
			continue
		}
		if strings.Contains(line, "Paired") {
			inPendingSection = false
			continue
		}

		if !inPendingSection {
			continue
		}

		// Look for UUID pattern (request ID) in the line
		parts := strings.Split(line, "│")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			// UUID format: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
			if len(part) == 36 && strings.Count(part, "-") == 4 {
				// This is a request ID, approve it
				ExecInPod(ctx, namespace, podName, "openclaw",
					[]string{"node", "/app/openclaw.mjs", "devices", "approve", part})
			}
		}
	}
}

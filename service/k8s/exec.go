package k8s

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

func ListOptions(deploymentName string) metav1.ListOptions {
	return metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app=openclaw,bot-id=%s", extractBotID(deploymentName)),
	}
}

func extractBotID(deploymentName string) string {
	// deploymentName format: openclaw-{botID}
	if len(deploymentName) > 9 {
		return deploymentName[9:]
	}
	return deploymentName
}

// WriteConfigToBot writes the openclaw.json config file to the bot's pod
func WriteConfigToBot(ctx context.Context, botID string, config *BotConfig) error {
	if config == nil {
		return nil
	}

	namespace := GetNamespace()

	// Wait for pod to be ready and get pod name
	podName, err := waitForPodReady(ctx, botID, 60) // 60 seconds timeout
	if err != nil {
		return fmt.Errorf("failed to wait for pod ready: %w", err)
	}

	// Build openclaw.json content
	configJSON := buildOpenClawConfig(config)

	// Write config file to OpenClaw's config directory
	// Note: OpenClaw container uses HOME=/home/node, so ~/.openclaw = /home/node/.openclaw
	command := []string{"sh", "-c", fmt.Sprintf("cat > /home/node/.openclaw/openclaw.json << 'EOFCONFIG'\n%s\nEOFCONFIG", configJSON)}

	_, err = ExecInPod(ctx, namespace, podName, "openclaw", command)
	if err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	// Set default model using CLI (config file alone doesn't work)
	providerName := "anthropic"
	if strings.Contains(config.BaseURL, "minimax") {
		providerName = "minimax"
	}
	model := config.Model
	if model == "" {
		model = "claude-sonnet-4-20250514"
	}
	fullModelID := fmt.Sprintf("%s/%s", providerName, model)

	_, err = ExecInPod(ctx, namespace, podName, "openclaw",
		[]string{"node", "/app/openclaw.mjs", "models", "set", fullModelID})
	if err != nil {
		// Log but don't fail - config file is still written
		fmt.Printf("warning: failed to set default model: %v\n", err)
	}

	// Restart deployment so OpenClaw reloads config from file
	// OpenClaw only reads config at startup, so we need to restart to apply changes
	if err := RestartDeployment(ctx, botID); err != nil {
		return fmt.Errorf("failed to restart deployment: %w", err)
	}

	// Wait for new pod to be ready after restart
	newPodName, err := waitForPodReady(ctx, botID, 120) // 120 seconds timeout for restart
	if err != nil {
		return fmt.Errorf("failed to wait for pod ready after restart: %w", err)
	}

	// Start background goroutine to auto-approve pairing requests
	go autoApprovePairingRequests(botID, newPodName)

	return nil
}

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

func buildOpenClawConfig(config *BotConfig) string {
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	model := config.Model
	if model == "" {
		model = "claude-sonnet-4-20250514"
	}

	// Determine provider name based on baseURL
	providerName := "anthropic"
	if strings.Contains(baseURL, "minimax") {
		providerName = "minimax"
	}

	return fmt.Sprintf(`{
  "gateway": {
    "mode": "local"
  },
  "models": {
    "mode": "replace",
    "providers": {
      "%s": {
        "baseUrl": "%s",
        "apiKey": "%s",
        "api": "anthropic-messages",
        "models": [
          {
            "id": "%s",
            "name": "%s",
            "reasoning": false,
            "input": ["text"],
            "contextWindow": 200000,
            "maxTokens": 8192
          }
        ]
      }
    }
  }
}`, providerName, baseURL, config.APIKey, model, model)
}

func waitForPodReady(ctx context.Context, botID string, timeoutSeconds int) (string, error) {
	client := GetClient()
	namespace := GetNamespace()

	for i := 0; i < timeoutSeconds; i++ {
		pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("app=openclaw,bot-id=%s", botID),
		})
		if err != nil {
			return "", err
		}

		for _, pod := range pods.Items {
			if pod.Status.Phase == corev1.PodRunning {
				for _, cond := range pod.Status.Conditions {
					if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
						return pod.Name, nil
					}
				}
			}
		}

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(time.Second):
			// Wait 1 second before retry
		}
	}

	return "", fmt.Errorf("timeout waiting for pod to be ready")
}


func ExecInPod(ctx context.Context, namespace, podName, containerName string, command []string) (string, error) {
	client := GetClient()
	config := GetRestConfig()

	if config == nil {
		return "", fmt.Errorf("rest config not initialized")
	}

	req := client.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: containerName,
			Command:   command,
			Stdin:     false,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return "", fmt.Errorf("failed to create executor: %w", err)
	}

	var stdout, stderr bytes.Buffer
	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if err != nil {
		return "", fmt.Errorf("exec failed: %w, stderr: %s", err, stderr.String())
	}

	return stdout.String(), nil
}

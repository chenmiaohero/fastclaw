package k8s

import (
	"context"
	"fmt"
	"strings"
)

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
	providerName := getProviderName(config.BaseURL)
	model := getDefaultModel(config.Model)
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

// getProviderName returns the provider name based on baseURL
func getProviderName(baseURL string) string {
	if strings.Contains(baseURL, "minimax") {
		return "minimax"
	}
	return "anthropic"
}

// getDefaultModel returns the model or a default value
func getDefaultModel(model string) string {
	if model == "" {
		return "claude-sonnet-4-20250514"
	}
	return model
}

// buildOpenClawConfig builds the openclaw.json configuration content
func buildOpenClawConfig(config *BotConfig) string {
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	model := getDefaultModel(config.Model)
	providerName := getProviderName(baseURL)

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

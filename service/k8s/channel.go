package k8s

import (
	"context"
	"fmt"
	"strings"
)

// AddChannelToBot adds an IM channel to a bot's OpenClaw instance
// This is hot-loaded, no restart needed
// accessToken is the bot's access token for gateway authentication
func AddChannelToBot(ctx context.Context, botID, accessToken, channel, token, botToken, appToken string) error {
	namespace := GetNamespace()

	// Get pod name
	podName, err := waitForPodReady(ctx, botID, 30)
	if err != nil {
		return fmt.Errorf("failed to get pod: %w", err)
	}

	// Build command based on channel type (all commands include gateway auth token)
	var command []string
	switch channel {
	case "telegram":
		command = []string{"node", "/app/openclaw.mjs", "channels", "add",
			"--channel", "telegram",
			"--token", token,
			"--gateway-token", accessToken}
	case "discord":
		command = []string{"node", "/app/openclaw.mjs", "channels", "add",
			"--channel", "discord",
			"--token", token,
			"--gateway-token", accessToken}
	case "slack":
		if botToken == "" || appToken == "" {
			return fmt.Errorf("slack requires both bot_token and app_token")
		}
		command = []string{"node", "/app/openclaw.mjs", "channels", "add",
			"--channel", "slack",
			"--bot-token", botToken,
			"--app-token", appToken,
			"--gateway-token", accessToken}
	case "whatsapp":
		// WhatsApp uses QR code auth, just initialize the channel
		command = []string{"node", "/app/openclaw.mjs", "channels", "add",
			"--channel", "whatsapp",
			"--gateway-token", accessToken}
	default:
		// Generic channel with token
		command = []string{"node", "/app/openclaw.mjs", "channels", "add",
			"--channel", channel,
			"--token", token,
			"--gateway-token", accessToken}
	}

	_, err = ExecInPod(ctx, namespace, podName, "openclaw", command)
	if err != nil {
		return fmt.Errorf("failed to add channel: %w", err)
	}

	return nil
}

// ListBotChannels lists all configured channels for a bot
func ListBotChannels(ctx context.Context, botID, accessToken string) ([]map[string]string, error) {
	namespace := GetNamespace()

	podName, err := waitForPodReady(ctx, botID, 30)
	if err != nil {
		return nil, fmt.Errorf("failed to get pod: %w", err)
	}

	output, err := ExecInPod(ctx, namespace, podName, "openclaw",
		[]string{"node", "/app/openclaw.mjs", "channels", "list", "--token", accessToken})
	if err != nil {
		return nil, fmt.Errorf("failed to list channels: %w", err)
	}

	// Parse output (simple text parsing)
	channels := parseChannelList(output)
	return channels, nil
}

// RemoveChannelFromBot removes an IM channel from a bot
func RemoveChannelFromBot(ctx context.Context, botID, accessToken, channel string) error {
	namespace := GetNamespace()

	podName, err := waitForPodReady(ctx, botID, 30)
	if err != nil {
		return fmt.Errorf("failed to get pod: %w", err)
	}

	_, err = ExecInPod(ctx, namespace, podName, "openclaw",
		[]string{"node", "/app/openclaw.mjs", "channels", "remove", "--channel", channel, "--token", accessToken})
	if err != nil {
		return fmt.Errorf("failed to remove channel: %w", err)
	}

	return nil
}

// parseChannelList parses the output of `openclaw channels list`
func parseChannelList(output string) []map[string]string {
	var channels []map[string]string
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Skip headers and empty lines
		if line == "" || strings.HasPrefix(line, "Chat channels") ||
			strings.HasPrefix(line, "Auth providers") ||
			strings.HasPrefix(line, "Usage") ||
			strings.HasPrefix(line, "Docs") ||
			strings.HasPrefix(line, "-") {
			continue
		}

		// Parse channel entries (format varies)
		if strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				channels = append(channels, map[string]string{
					"channel": strings.TrimSpace(parts[0]),
					"status":  strings.TrimSpace(parts[1]),
				})
			}
		}
	}

	return channels
}

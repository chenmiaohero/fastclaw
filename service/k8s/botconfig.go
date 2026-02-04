package k8s

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// WriteConfigToBot writes the openclaw.json config file to the bot's pod
// If forceSetDefaultModel is true, always set agents.defaults.model.primary
func WriteConfigToBot(ctx context.Context, botID string, config *BotConfig, forceSetDefaultModel bool) error {
	if config == nil {
		return nil
	}

	namespace := GetNamespace()

	// Wait for pod to be ready and get pod name
	podName, err := waitForPodReady(ctx, botID, 60) // 60 seconds timeout
	if err != nil {
		return fmt.Errorf("failed to wait for pod ready: %w", err)
	}

	// Determine whether to set default model
	setDefaultModel := forceSetDefaultModel
	if !forceSetDefaultModel {
		// Check if user has already configured a default model
		hasDefaultModel := checkHasDefaultModel(ctx, namespace, podName)
		setDefaultModel = !hasDefaultModel
	}

	// Build openclaw.json content
	configJSON := buildOpenClawConfig(config, setDefaultModel)

	// Write config file to OpenClaw's config directory
	// Note: OpenClaw container uses HOME=/home/node, so ~/.openclaw = /home/node/.openclaw
	command := []string{"sh", "-c", fmt.Sprintf("cat > /home/node/.openclaw/openclaw.json << 'EOFCONFIG'\n%s\nEOFCONFIG", configJSON)}

	_, err = ExecInPod(ctx, namespace, podName, "openclaw", command)
	if err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	// OpenClaw will auto-detect config changes and hot-reload (sends SIGUSR1 to restart gateway)
	// API only configures fallback provider, sets default model only if user hasn't configured one

	return nil
}

// checkHasDefaultModel checks if user has already configured a default model
func checkHasDefaultModel(ctx context.Context, namespace, podName string) bool {
	// Read existing config and check for agents.defaults.model.primary
	output, err := ExecInPod(ctx, namespace, podName, "openclaw",
		[]string{"sh", "-c", "cat /home/node/.openclaw/openclaw.json 2>/dev/null || echo '{}'"})
	if err != nil {
		return false
	}
	// Simple check: if config contains "agents" with "defaults", user likely has configured it
	return strings.Contains(output, `"agents"`) && strings.Contains(output, `"primary"`)
}

// getProviderName returns the provider name, using explicit provider if set, otherwise inferred from baseURL
func getProviderName(provider, baseURL string) string {
	// Use explicit provider if specified
	if provider != "" {
		return provider
	}
	// Otherwise infer from baseURL
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

// getTrustedProxies returns the trusted proxy list from config
// Defaults to private network ranges if not configured
func getTrustedProxies() string {
	proxies := viper.GetStringSlice("openclaw.trusted_proxies")
	if len(proxies) == 0 {
		// Default: trust all private network ranges (RFC 1918)
		// This allows proxies from 10.x.x.x, 172.16-31.x.x, 192.168.x.x
		proxies = []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "127.0.0.0/8"}
	}
	// Format as JSON array
	quoted := make([]string, len(proxies))
	for i, p := range proxies {
		quoted[i] = fmt.Sprintf(`"%s"`, p)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

// getAllowedOrigins returns the allowed origins list from config
// Returns empty string if not configured (OpenClaw may not support this in all versions)
func getAllowedOrigins() string {
	origins := viper.GetStringSlice("openclaw.allowed_origins")
	if len(origins) == 0 {
		// Don't set allowedOrigins by default - some OpenClaw versions don't support it
		// The proxy already rewrites Origin header to bypass origin checks
		return ""
	}
	// Format as JSON array
	quoted := make([]string, len(origins))
	for i, o := range origins {
		quoted[i] = fmt.Sprintf(`"%s"`, o)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

// getGatewayPort returns the gateway port from config
func getGatewayPort() int {
	port := viper.GetInt("openclaw.gateway_port")
	if port == 0 {
		return 18789
	}
	return port
}

// buildOpenClawConfig builds the openclaw.json configuration content
// setDefaultModel: if true, also sets agents.defaults.model.primary (for first-time setup)
func buildOpenClawConfig(config *BotConfig, setDefaultModel bool) string {
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	model := getDefaultModel(config.Model)
	providerName := getProviderName(config.Provider, baseURL)
	fullModelID := fmt.Sprintf("%s/%s", providerName, model)

	// Default auth and api values for model provider
	auth := config.Auth
	if auth == "" {
		auth = "api-key"
	}
	api := config.API
	if api == "" {
		api = "anthropic-messages"
	}

	// Build gateway section with password auth
	gatewayPort := getGatewayPort()
	trustedProxies := getTrustedProxies()
	allowedOrigins := getAllowedOrigins()

	// Build optional gateway parts
	var optionalParts string
	if trustedProxies != "" {
		optionalParts += fmt.Sprintf(",\n    \"trustedProxies\": %s", trustedProxies)
	}
	if allowedOrigins != "" {
		optionalParts += fmt.Sprintf(",\n    \"controlUi\": {\n      \"allowedOrigins\": %s\n    }", allowedOrigins)
	}

	gatewaySection := fmt.Sprintf(`"gateway": {
    "port": %d,
    "mode": "local",
    "bind": "lan",
    "auth": {
      "mode": "password",
      "password": "%s"
    },
    "tailscale": {
      "mode": "off",
      "resetOnExit": false
    }%s
  }`, gatewayPort, config.Password, optionalParts)

	if setDefaultModel {
		// Include agents.defaults to set fallback model for first-time users
		return fmt.Sprintf(`{
  %s,
  "agents": {
    "defaults": {
      "model": {
        "primary": "%s"
      }
    }
  },
  "models": {
    "mode": "merge",
    "providers": {
      "%s": {
        "baseUrl": "%s",
        "apiKey": "%s",
        "auth": "%s",
        "api": "%s",
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
}`, gatewaySection, fullModelID, providerName, baseURL, config.APIKey, auth, api, model, model)
	}

	// Only add provider, don't change user's default model
	return fmt.Sprintf(`{
  %s,
  "models": {
    "mode": "merge",
    "providers": {
      "%s": {
        "baseUrl": "%s",
        "apiKey": "%s",
        "auth": "%s",
        "api": "%s",
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
}`, gatewaySection, providerName, baseURL, config.APIKey, auth, api, model, model)
}

// BuildGatewayConfig builds the minimal gateway config for initial startup
// This is used to write config before gateway starts (in container command)
func BuildGatewayConfig(config *BotConfig, port int32) string {
	trustedProxies := getTrustedProxies()
	allowedOrigins := getAllowedOrigins()

	// Build optional parts
	var optionalParts string
	if trustedProxies != "" {
		optionalParts += fmt.Sprintf(`,
    "trustedProxies": %s`, trustedProxies)
	}
	if allowedOrigins != "" {
		optionalParts += fmt.Sprintf(`,
    "controlUi": {
      "allowedOrigins": %s
    }`, allowedOrigins)
	}

	return fmt.Sprintf(`{
  "gateway": {
    "port": %d,
    "mode": "local",
    "bind": "lan",
    "auth": {
      "mode": "password",
      "password": "%s"
    },
    "tailscale": {
      "mode": "off",
      "resetOnExit": false
    }%s
  }
}`, port, config.Password, optionalParts)
}

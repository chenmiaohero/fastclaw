package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/fastclaw-ai/fastclaw/model"
)

// ReadOpenClawConfig reads the openclaw.json config from the pod
func ReadOpenClawConfig(ctx context.Context, botID string) (*model.OpenClawConfig, error) {
	namespace := GetNamespace()

	podName, err := waitForPodReady(ctx, botID, 30)
	if err != nil {
		return nil, fmt.Errorf("failed to get pod: %w", err)
	}

	output, err := ExecInPod(ctx, namespace, podName, "openclaw",
		[]string{"cat", "/home/node/.openclaw/openclaw.json"})
	if err != nil {
		// If file doesn't exist, return empty config
		if strings.Contains(err.Error(), "No such file") {
			return &model.OpenClawConfig{}, nil
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var config model.OpenClawConfig
	if err := json.Unmarshal([]byte(output), &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &config, nil
}

// SyncConfigToDatabase reads the openclaw config from pod and saves to database
func SyncConfigToDatabase(ctx context.Context, botID string) error {
	// Read config from pod
	config, err := ReadOpenClawConfig(ctx, botID)
	if err != nil {
		return fmt.Errorf("failed to read config from pod: %w", err)
	}

	// Get bot from database
	bot, err := model.GetBotByID(botID)
	if err != nil {
		return fmt.Errorf("failed to get bot: %w", err)
	}

	// Update bot config
	if err := bot.SetOpenClawConfig(config); err != nil {
		return fmt.Errorf("failed to set config: %w", err)
	}

	// Save to database
	if err := model.UpdateBot(bot); err != nil {
		return fmt.Errorf("failed to update bot: %w", err)
	}

	return nil
}

// WriteOpenClawConfigToPod writes the full openclaw config to the pod
func WriteOpenClawConfigToPod(ctx context.Context, botID string, config *model.OpenClawConfig) error {
	namespace := GetNamespace()

	podName, err := waitForPodReady(ctx, botID, 60)
	if err != nil {
		return fmt.Errorf("failed to get pod: %w", err)
	}

	configJSON, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	command := []string{"sh", "-c", fmt.Sprintf("cat > /home/node/.openclaw/openclaw.json << 'EOFCONFIG'\n%s\nEOFCONFIG", string(configJSON))}

	_, err = ExecInPod(ctx, namespace, podName, "openclaw", command)
	if err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// SyncConfigToPod reads config from database and writes to pod
func SyncConfigToPod(ctx context.Context, botID string) error {
	// Get bot from database
	bot, err := model.GetBotByID(botID)
	if err != nil {
		return fmt.Errorf("failed to get bot: %w", err)
	}

	// Get config
	config, err := bot.GetOpenClawConfig()
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	// Ensure gateway auth is set
	if config.Gateway == nil {
		config.Gateway = &model.GatewayConfig{}
	}
	if config.Gateway.Auth == nil {
		config.Gateway.Auth = &model.GatewayAuthConfig{}
	}

	// Set gateway auth using token auth with AccessToken
	// Always update token when mode is "token" or empty (to handle token reset)
	// Password auth is configured via config.gateway.auth in OpenClaw config
	if config.Gateway.Auth.Mode == "" || config.Gateway.Auth.Mode == "token" {
		config.Gateway.Auth.Mode = "token"
		config.Gateway.Auth.Token = bot.AccessToken
	}

	// Write to pod
	return WriteOpenClawConfigToPod(ctx, botID, config)
}

// UpdateModelsConfig updates only the models section and syncs
func UpdateModelsConfig(ctx context.Context, botID string, modelsConfig *model.ModelsConfig) error {
	// Get bot from database
	bot, err := model.GetBotByID(botID)
	if err != nil {
		return fmt.Errorf("failed to get bot: %w", err)
	}

	// Get existing config
	config, err := bot.GetOpenClawConfig()
	if err != nil {
		config = &model.OpenClawConfig{}
	}

	// Update models section
	config.Models = modelsConfig

	// Save to database
	if err := bot.SetOpenClawConfig(config); err != nil {
		return fmt.Errorf("failed to set config: %w", err)
	}
	if err := model.UpdateBot(bot); err != nil {
		return fmt.Errorf("failed to update bot: %w", err)
	}

	// If bot is running, sync to pod
	if bot.Status == model.BotStatusRunning {
		return SyncConfigToPod(ctx, botID)
	}

	return nil
}

// UpdateAgentsConfig updates only the agents section and syncs
func UpdateAgentsConfig(ctx context.Context, botID string, agentsConfig *model.AgentsConfig) error {
	// Get bot from database
	bot, err := model.GetBotByID(botID)
	if err != nil {
		return fmt.Errorf("failed to get bot: %w", err)
	}

	// Get existing config
	config, err := bot.GetOpenClawConfig()
	if err != nil {
		config = &model.OpenClawConfig{}
	}

	// Update agents section
	config.Agents = agentsConfig

	// Save to database
	if err := bot.SetOpenClawConfig(config); err != nil {
		return fmt.Errorf("failed to set config: %w", err)
	}
	if err := model.UpdateBot(bot); err != nil {
		return fmt.Errorf("failed to update bot: %w", err)
	}

	// If bot is running, sync to pod
	if bot.Status == model.BotStatusRunning {
		return SyncConfigToPod(ctx, botID)
	}

	return nil
}

// UpdateChannelsConfig updates only the channels section and syncs
func UpdateChannelsConfig(ctx context.Context, botID string, channelsConfig model.ChannelsConfig) error {
	// Get bot from database
	bot, err := model.GetBotByID(botID)
	if err != nil {
		return fmt.Errorf("failed to get bot: %w", err)
	}

	// Get existing config
	config, err := bot.GetOpenClawConfig()
	if err != nil {
		config = &model.OpenClawConfig{}
	}

	// Update channels section
	config.Channels = channelsConfig

	// Save to database
	if err := bot.SetOpenClawConfig(config); err != nil {
		return fmt.Errorf("failed to set config: %w", err)
	}
	if err := model.UpdateBot(bot); err != nil {
		return fmt.Errorf("failed to update bot: %w", err)
	}

	// If bot is running, sync to pod
	if bot.Status == model.BotStatusRunning {
		return SyncConfigToPod(ctx, botID)
	}

	return nil
}

// AddChannel adds a channel to the config and syncs
func AddChannel(ctx context.Context, botID, channelName string, channelConfig *model.ChannelConfig) error {
	// Get bot from database
	bot, err := model.GetBotByID(botID)
	if err != nil {
		return fmt.Errorf("failed to get bot: %w", err)
	}

	// Get existing config
	config, err := bot.GetOpenClawConfig()
	if err != nil {
		config = &model.OpenClawConfig{}
	}

	// Ensure channels map exists
	if config.Channels == nil {
		config.Channels = make(model.ChannelsConfig)
	}

	// Add/update channel
	channelConfig.Enabled = true
	config.Channels[channelName] = channelConfig

	// Save to database
	if err := bot.SetOpenClawConfig(config); err != nil {
		return fmt.Errorf("failed to set config: %w", err)
	}
	if err := model.UpdateBot(bot); err != nil {
		return fmt.Errorf("failed to update bot: %w", err)
	}

	// If bot is running, sync to pod
	if bot.Status == model.BotStatusRunning {
		return SyncConfigToPod(ctx, botID)
	}

	return nil
}

// RemoveChannel removes a channel from the config and syncs
func RemoveChannel(ctx context.Context, botID, channelName string) error {
	// Get bot from database
	bot, err := model.GetBotByID(botID)
	if err != nil {
		return fmt.Errorf("failed to get bot: %w", err)
	}

	// Get existing config
	config, err := bot.GetOpenClawConfig()
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	// Remove channel
	if config.Channels != nil {
		delete(config.Channels, channelName)
	}

	// Save to database
	if err := bot.SetOpenClawConfig(config); err != nil {
		return fmt.Errorf("failed to set config: %w", err)
	}
	if err := model.UpdateBot(bot); err != nil {
		return fmt.Errorf("failed to update bot: %w", err)
	}

	// If bot is running, sync to pod
	if bot.Status == model.BotStatusRunning {
		return SyncConfigToPod(ctx, botID)
	}

	return nil
}

// AddOrUpdateProvider adds or updates a provider in the models config
func AddOrUpdateProvider(ctx context.Context, botID, providerName string, providerConfig *model.ProviderConfig) error {
	// Get bot from database
	bot, err := model.GetBotByID(botID)
	if err != nil {
		return fmt.Errorf("failed to get bot: %w", err)
	}

	// Get existing config
	config, err := bot.GetOpenClawConfig()
	if err != nil {
		config = &model.OpenClawConfig{}
	}

	// Ensure models section exists
	if config.Models == nil {
		config.Models = &model.ModelsConfig{
			Mode:      "merge",
			Providers: make(map[string]*model.ProviderConfig),
		}
	}
	if config.Models.Providers == nil {
		config.Models.Providers = make(map[string]*model.ProviderConfig)
	}

	// Add/update provider
	config.Models.Providers[providerName] = providerConfig

	// Save to database
	if err := bot.SetOpenClawConfig(config); err != nil {
		return fmt.Errorf("failed to set config: %w", err)
	}
	if err := model.UpdateBot(bot); err != nil {
		return fmt.Errorf("failed to update bot: %w", err)
	}

	// If bot is running, sync to pod
	if bot.Status == model.BotStatusRunning {
		return SyncConfigToPod(ctx, botID)
	}

	return nil
}

// RemoveProvider removes a provider from the models config
func RemoveProvider(ctx context.Context, botID, providerName string) error {
	// Get bot from database
	bot, err := model.GetBotByID(botID)
	if err != nil {
		return fmt.Errorf("failed to get bot: %w", err)
	}

	// Get existing config
	config, err := bot.GetOpenClawConfig()
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	// Remove provider
	if config.Models != nil && config.Models.Providers != nil {
		delete(config.Models.Providers, providerName)
	}

	// Save to database
	if err := bot.SetOpenClawConfig(config); err != nil {
		return fmt.Errorf("failed to set config: %w", err)
	}
	if err := model.UpdateBot(bot); err != nil {
		return fmt.Errorf("failed to update bot: %w", err)
	}

	// If bot is running, sync to pod
	if bot.Status == model.BotStatusRunning {
		return SyncConfigToPod(ctx, botID)
	}

	return nil
}

// SetDefaultModel sets the default model in agents config
func SetDefaultModel(ctx context.Context, botID, primaryModel string) error {
	// Get bot from database
	bot, err := model.GetBotByID(botID)
	if err != nil {
		return fmt.Errorf("failed to get bot: %w", err)
	}

	// Get existing config
	config, err := bot.GetOpenClawConfig()
	if err != nil {
		config = &model.OpenClawConfig{}
	}

	// Ensure agents section exists
	if config.Agents == nil {
		config.Agents = &model.AgentsConfig{}
	}
	if config.Agents.Defaults == nil {
		config.Agents.Defaults = &model.AgentDefaultsConfig{}
	}
	if config.Agents.Defaults.Model == nil {
		config.Agents.Defaults.Model = &model.AgentModelConfig{}
	}

	// Set primary model
	config.Agents.Defaults.Model.Primary = primaryModel

	// Save to database
	if err := bot.SetOpenClawConfig(config); err != nil {
		return fmt.Errorf("failed to set config: %w", err)
	}
	if err := model.UpdateBot(bot); err != nil {
		return fmt.Errorf("failed to update bot: %w", err)
	}

	// If bot is running, sync to pod
	if bot.Status == model.BotStatusRunning {
		return SyncConfigToPod(ctx, botID)
	}

	return nil
}

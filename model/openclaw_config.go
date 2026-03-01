package model

import (
	"encoding/json"
)

// OpenClawConfig represents the full openclaw.json configuration format
// This structure matches the openclaw configuration file format
type OpenClawConfig struct {
	Meta     *MetaConfig     `json:"meta,omitempty"`
	Models   *ModelsConfig   `json:"models,omitempty"`
	Agents   *AgentsConfig   `json:"agents,omitempty"`
	Channels ChannelsConfig  `json:"channels,omitempty"`
	Gateway  *GatewayConfig  `json:"gateway,omitempty"`
	Auth     *AuthConfig     `json:"auth,omitempty"`
	Plugins  *PluginsConfig  `json:"plugins,omitempty"`
	Messages *MessagesConfig `json:"messages,omitempty"`
	Commands *CommandsConfig `json:"commands,omitempty"`
	Hooks    *HooksConfig    `json:"hooks,omitempty"`
}

// MetaConfig represents metadata about the configuration
type MetaConfig struct {
	LastTouchedVersion string `json:"lastTouchedVersion,omitempty"`
	LastTouchedAt      string `json:"lastTouchedAt,omitempty"`
}

// ModelsConfig represents the models configuration
type ModelsConfig struct {
	Mode      string                        `json:"mode,omitempty"` // "merge" or "replace"
	Providers map[string]*ProviderConfig    `json:"providers,omitempty"`
}

// ProviderConfig represents a single provider configuration
type ProviderConfig struct {
	BaseURL    string                `json:"baseUrl,omitempty"`
	APIKey     string                `json:"apiKey,omitempty"`
	Auth       string                `json:"auth,omitempty"`       // api-key, bearer
	AuthHeader bool                  `json:"authHeader,omitempty"` // whether to send API key in Authorization header
	API        string                `json:"api,omitempty"`        // anthropic-messages, openai-completions
	Models     []ProviderModelConfig `json:"models,omitempty"`
}

// ProviderModelConfig represents a model configuration within a provider
type ProviderModelConfig struct {
	ID            string   `json:"id"`
	Name          string   `json:"name,omitempty"`
	Reasoning     bool     `json:"reasoning,omitempty"`
	Input         []string `json:"input,omitempty"`
	ContextWindow int      `json:"contextWindow,omitempty"`
	MaxTokens     int      `json:"maxTokens,omitempty"`
}

// AgentsConfig represents the agents configuration
type AgentsConfig struct {
	Defaults *AgentDefaultsConfig `json:"defaults,omitempty"`
}

// AgentDefaultsConfig represents agent default settings
type AgentDefaultsConfig struct {
	Model         *AgentModelConfig            `json:"model,omitempty"`
	Models        map[string]*AgentModelAlias  `json:"models,omitempty"`
	Workspace     string                       `json:"workspace,omitempty"`
	Compaction    *CompactionConfig            `json:"compaction,omitempty"`
	MaxConcurrent int                          `json:"maxConcurrent,omitempty"`
	Subagents     *SubagentsConfig             `json:"subagents,omitempty"`
}

// AgentModelConfig represents the primary model configuration
type AgentModelConfig struct {
	Primary string `json:"primary,omitempty"` // e.g., "anthropic/claude-sonnet-4-20250514"
}

// AgentModelAlias represents a model alias configuration
type AgentModelAlias struct {
	Alias string `json:"alias,omitempty"`
}

// CompactionConfig represents compaction settings
type CompactionConfig struct {
	Mode string `json:"mode,omitempty"` // "safeguard", etc.
}

// SubagentsConfig represents subagent settings
type SubagentsConfig struct {
	MaxConcurrent int `json:"maxConcurrent,omitempty"`
}

// ChannelsConfig is a map of channel name to channel configuration
// Using interface{} to support various channel config formats (accounts, botToken, etc.)
type ChannelsConfig map[string]interface{}

// ChannelConfig represents a single channel configuration
type ChannelConfig struct {
	Enabled        bool                   `json:"enabled,omitempty"`
	BotToken       string                 `json:"botToken,omitempty"`       // Telegram, Discord
	AppToken       string                 `json:"appToken,omitempty"`       // Slack
	AppID          string                 `json:"appId,omitempty"`          // Feishu, Teams
	AppSecret      string                 `json:"appSecret,omitempty"`      // Feishu
	AppPassword    string                 `json:"appPassword,omitempty"`    // Teams
	ChannelSecret  string                 `json:"channelSecret,omitempty"`  // LINE
	DMPolicy       string                 `json:"dmPolicy,omitempty"`       // pairing, allowlist, open, disabled
	GroupPolicy    string                 `json:"groupPolicy,omitempty"`    // open, allowlist, disabled
	TextChunkLimit int                    `json:"textChunkLimit,omitempty"`
	MediaMaxMb     int                    `json:"mediaMaxMb,omitempty"`
	Extra          map[string]interface{} `json:"-"`                        // Additional fields
}

// MarshalJSON implements custom JSON marshaling for ChannelConfig
func (c *ChannelConfig) MarshalJSON() ([]byte, error) {
	type Alias ChannelConfig
	data, err := json.Marshal((*Alias)(c))
	if err != nil {
		return nil, err
	}

	if len(c.Extra) == 0 {
		return data, nil
	}

	// Merge extra fields
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	for k, v := range c.Extra {
		if _, exists := m[k]; !exists {
			m[k] = v
		}
	}
	return json.Marshal(m)
}

// UnmarshalJSON implements custom JSON unmarshaling for ChannelConfig
func (c *ChannelConfig) UnmarshalJSON(data []byte) error {
	type Alias ChannelConfig
	if err := json.Unmarshal(data, (*Alias)(c)); err != nil {
		return err
	}

	// Capture unknown fields in Extra
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}

	knownFields := map[string]bool{
		"enabled": true, "botToken": true, "appToken": true, "appId": true,
		"appSecret": true, "appPassword": true, "channelSecret": true,
		"dmPolicy": true, "groupPolicy": true, "textChunkLimit": true, "mediaMaxMb": true,
	}

	c.Extra = make(map[string]interface{})
	for k, v := range m {
		if !knownFields[k] {
			c.Extra[k] = v
		}
	}
	return nil
}

// GatewayConfig represents gateway configuration
type GatewayConfig struct {
	Port           int                    `json:"port,omitempty"`
	Mode           string                 `json:"mode,omitempty"` // local, remote
	Bind           string                 `json:"bind,omitempty"` // loopback, lan
	Auth           *GatewayAuthConfig     `json:"auth,omitempty"`
	Tailscale      *TailscaleConfig       `json:"tailscale,omitempty"`
	TrustedProxies []string               `json:"trustedProxies,omitempty"`
	ControlUI      *ControlUIConfig       `json:"controlUi,omitempty"`
}

// GatewayAuthConfig represents gateway authentication configuration
type GatewayAuthConfig struct {
	Mode     string `json:"mode,omitempty"` // password, token
	Password string `json:"password,omitempty"`
	Token    string `json:"token,omitempty"`
}

// TailscaleConfig represents Tailscale configuration
type TailscaleConfig struct {
	Mode        string `json:"mode,omitempty"` // off, on
	ResetOnExit bool   `json:"resetOnExit,omitempty"`
}

// ControlUIConfig represents control UI configuration
type ControlUIConfig struct {
	AllowedOrigins                []string `json:"allowedOrigins,omitempty"`
	DangerouslyDisableDeviceAuth  bool     `json:"dangerouslyDisableDeviceAuth,omitempty"`
}

// AuthConfig represents authentication profiles configuration
type AuthConfig struct {
	Profiles map[string]*AuthProfile `json:"profiles,omitempty"`
}

// AuthProfile represents an authentication profile
type AuthProfile struct {
	Provider string `json:"provider,omitempty"`
	Mode     string `json:"mode,omitempty"`
}

// PluginsConfig represents plugins configuration
type PluginsConfig struct {
	Entries map[string]interface{} `json:"entries,omitempty"`
}

// MessagesConfig represents messages configuration
type MessagesConfig struct {
	AckReactionScope string `json:"ackReactionScope,omitempty"`
}

// CommandsConfig represents commands configuration
type CommandsConfig struct {
	Native       string `json:"native,omitempty"`
	NativeSkills string `json:"nativeSkills,omitempty"`
}

// HooksConfig represents hooks configuration
type HooksConfig struct {
	Internal *InternalHooksConfig `json:"internal,omitempty"`
}

// InternalHooksConfig represents internal hooks configuration
type InternalHooksConfig struct {
	Enabled bool                          `json:"enabled,omitempty"`
	Entries map[string]*HookEntryConfig   `json:"entries,omitempty"`
}

// HookEntryConfig represents a hook entry configuration
type HookEntryConfig struct {
	Enabled bool `json:"enabled,omitempty"`
}

// GetBotOpenClawConfig returns the OpenClaw configuration from bot.Config
func (b *Bot) GetOpenClawConfig() (*OpenClawConfig, error) {
	if b.Config == nil {
		return &OpenClawConfig{}, nil
	}
	var config OpenClawConfig
	if err := json.Unmarshal(b.Config, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

// SetOpenClawConfig sets the OpenClaw configuration to bot.Config
func (b *Bot) SetOpenClawConfig(config *OpenClawConfig) error {
	data, err := json.Marshal(config)
	if err != nil {
		return err
	}
	b.Config = data
	return nil
}

// MergeOpenClawConfig merges partial config into existing config
func (b *Bot) MergeOpenClawConfig(partial *OpenClawConfig) error {
	existing, err := b.GetOpenClawConfig()
	if err != nil {
		existing = &OpenClawConfig{}
	}

	// Merge models
	if partial.Models != nil {
		if existing.Models == nil {
			existing.Models = &ModelsConfig{}
		}
		if partial.Models.Mode != "" {
			existing.Models.Mode = partial.Models.Mode
		}
		if partial.Models.Providers != nil {
			if existing.Models.Providers == nil {
				existing.Models.Providers = make(map[string]*ProviderConfig)
			}
			for name, provider := range partial.Models.Providers {
				existing.Models.Providers[name] = provider
			}
		}
	}

	// Merge agents
	if partial.Agents != nil {
		if existing.Agents == nil {
			existing.Agents = &AgentsConfig{}
		}
		if partial.Agents.Defaults != nil {
			if existing.Agents.Defaults == nil {
				existing.Agents.Defaults = &AgentDefaultsConfig{}
			}
			if partial.Agents.Defaults.Model != nil {
				existing.Agents.Defaults.Model = partial.Agents.Defaults.Model
			}
			if partial.Agents.Defaults.Models != nil {
				existing.Agents.Defaults.Models = partial.Agents.Defaults.Models
			}
			if partial.Agents.Defaults.MaxConcurrent > 0 {
				existing.Agents.Defaults.MaxConcurrent = partial.Agents.Defaults.MaxConcurrent
			}
		}
	}

	// Merge channels
	if partial.Channels != nil {
		if existing.Channels == nil {
			existing.Channels = make(ChannelsConfig)
		}
		for name, channel := range partial.Channels {
			existing.Channels[name] = channel
		}
	}

	// Merge gateway
	if partial.Gateway != nil {
		existing.Gateway = partial.Gateway
	}

	return b.SetOpenClawConfig(existing)
}

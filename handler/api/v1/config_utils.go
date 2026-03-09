package v1

import (
	"github.com/clawhost/clawhost/model"
	"github.com/clawhost/clawhost/service/k8s"
)

// convertToK8sConfig converts model.OpenClawConfig to k8s.BotConfig
// This is used when starting a bot to pass config to K8s deployment
func convertToK8sConfig(bot *model.Bot, config *model.OpenClawConfig) *k8s.BotConfig {
	k8sConfig := &k8s.BotConfig{
		AccessToken: bot.AccessToken,
	}

	// Convert models/providers
	if config != nil && config.Models != nil && len(config.Models.Providers) > 0 {
		k8sConfig.Providers = make([]k8s.ModelProviderConfig, 0, len(config.Models.Providers))
		for name, p := range config.Models.Providers {
			provider := k8s.ModelProviderConfig{
				Name:       name,
				BaseURL:    p.BaseURL,
				APIKey:     p.APIKey,
				Auth:       p.Auth,
				AuthHeader: p.AuthHeader,
				API:        p.API,
			}
			if len(p.Models) > 0 {
				provider.Models = make([]k8s.ModelConfigEntry, len(p.Models))
				for j, m := range p.Models {
					provider.Models[j] = k8s.ModelConfigEntry{
						ID:            m.ID,
						Name:          m.Name,
						Reasoning:     m.Reasoning,
						Input:         m.Input,
						ContextWindow: m.ContextWindow,
						MaxTokens:     m.MaxTokens,
					}
				}
			}
			k8sConfig.Providers = append(k8sConfig.Providers, provider)
		}
	}

	// Convert agent defaults
	if config != nil && config.Agents != nil && config.Agents.Defaults != nil {
		if config.Agents.Defaults.Model != nil {
			k8sConfig.AgentDefaults = &k8s.AgentDefaultsConfig{
				PrimaryModel: config.Agents.Defaults.Model.Primary,
			}
			// Convert fallback model
			if config.Agents.Defaults.Models != nil {
				if fallback, ok := config.Agents.Defaults.Models["fallback"]; ok {
					k8sConfig.AgentDefaults.FallbackModel = fallback.Alias
				}
			}
		}
	}

	// Convert channels
	if config != nil && config.Channels != nil {
		k8sConfig.Channels = make(map[string]interface{})
		for name, ch := range config.Channels {
			k8sConfig.Channels[name] = ch
		}
	}

	return k8sConfig
}

// convertLegacyToK8sConfig converts legacy model.BotConfig to k8s.BotConfig
// This is for backward compatibility with old config format
func convertLegacyToK8sConfig(bot *model.Bot, config *model.BotConfig) *k8s.BotConfig {
	k8sConfig := &k8s.BotConfig{
		AccessToken: bot.AccessToken,
		// Legacy fields
		Provider: config.Provider,
		Model:    config.Model,
		APIKey:   config.APIKey,
		BaseURL:  config.BaseURL,
		Auth:     config.Auth,
		API:      config.API,
	}

	// Convert providers
	if len(config.Providers) > 0 {
		k8sConfig.Providers = make([]k8s.ModelProviderConfig, len(config.Providers))
		for i, p := range config.Providers {
			k8sConfig.Providers[i] = k8s.ModelProviderConfig{
				Name:    p.Name,
				BaseURL: p.BaseURL,
				APIKey:  p.APIKey,
				Auth:    p.Auth,
				API:     p.API,
			}
			if len(p.Models) > 0 {
				k8sConfig.Providers[i].Models = make([]k8s.ModelConfigEntry, len(p.Models))
				for j, m := range p.Models {
					k8sConfig.Providers[i].Models[j] = k8s.ModelConfigEntry{
						ID:            m.ID,
						Name:          m.Name,
						Reasoning:     m.Reasoning,
						Input:         m.Input,
						ContextWindow: m.ContextWindow,
						MaxTokens:     m.MaxTokens,
					}
				}
			}
		}
	}

	// Convert agent defaults
	if config.AgentDefaults != nil {
		k8sConfig.AgentDefaults = &k8s.AgentDefaultsConfig{
			PrimaryModel:  config.AgentDefaults.PrimaryModel,
			FallbackModel: config.AgentDefaults.FallbackModel,
		}
	}

	// Convert channels
	if len(config.Channels) > 0 {
		k8sConfig.Channels = config.Channels
	}

	return k8sConfig
}

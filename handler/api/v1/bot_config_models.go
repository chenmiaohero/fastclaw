package v1

import (
	"context"

	"github.com/labstack/echo/v4"
	"github.com/workany-ai/clawork/model"
	"github.com/workany-ai/clawork/service/k8s"
	"github.com/workany-ai/clawork/util"
	"gorm.io/gorm"
)

// ProviderRequest represents a request to add/update a provider
type ProviderRequest struct {
	Name    string                       `json:"name"`
	BaseURL string                       `json:"baseUrl,omitempty"`
	APIKey  string                       `json:"apiKey,omitempty"`
	Auth    string                       `json:"auth,omitempty"`
	API     string                       `json:"api,omitempty"`
	Models  []model.ProviderModelConfig  `json:"models,omitempty"`
}

// ListModelProviders returns all model providers for a bot
// GET /bots/:id/config/models
func ListModelProviders(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return util.BadRequest(c, "id is required")
	}

	bot, err := model.GetBotByID(id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return util.NotFound(c, "bot not found")
		}
		return util.InternalError(c, "failed to get bot")
	}

	config, err := bot.GetOpenClawConfig()
	if err != nil {
		return util.InternalError(c, "failed to get bot config")
	}

	// Return providers map
	providers := make(map[string]*model.ProviderConfig)
	if config.Models != nil && config.Models.Providers != nil {
		providers = config.Models.Providers
	}

	return util.Success(c, providers)
}

// AddModelProvider adds a new model provider to the bot
// POST /bots/:id/config/models
func AddModelProvider(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return util.BadRequest(c, "id is required")
	}

	var req ProviderRequest
	if err := c.Bind(&req); err != nil {
		return util.BadRequest(c, "invalid request body")
	}

	if req.Name == "" {
		return util.BadRequest(c, "provider name is required")
	}

	bot, err := model.GetBotByID(id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return util.NotFound(c, "bot not found")
		}
		return util.InternalError(c, "failed to get bot")
	}

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

	// Check if provider already exists
	if _, exists := config.Models.Providers[req.Name]; exists {
		return util.BadRequest(c, "provider already exists")
	}

	// Add provider
	config.Models.Providers[req.Name] = &model.ProviderConfig{
		BaseURL: req.BaseURL,
		APIKey:  req.APIKey,
		Auth:    req.Auth,
		API:     req.API,
		Models:  req.Models,
	}

	// Save to database
	if err := bot.SetOpenClawConfig(config); err != nil {
		return util.InternalError(c, "failed to set config")
	}
	if err := model.UpdateBot(bot); err != nil {
		return util.InternalError(c, "failed to update bot")
	}

	// Sync to pod if bot is running
	if bot.Status == model.BotStatusRunning {
		go func() {
			ctx := context.Background()
			if err := k8s.SyncConfigToPod(ctx, bot.ID); err != nil {
				c.Logger().Errorf("failed to sync config to pod: %v", err)
			}
		}()
	}

	return util.Success(c, config.Models.Providers[req.Name])
}

// GetModelProvider returns a single model provider by name
// GET /bots/:id/config/models/:provider
func GetModelProvider(c echo.Context) error {
	id := c.Param("id")
	providerName := c.Param("provider")
	if id == "" {
		return util.BadRequest(c, "id is required")
	}
	if providerName == "" {
		return util.BadRequest(c, "provider name is required")
	}

	bot, err := model.GetBotByID(id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return util.NotFound(c, "bot not found")
		}
		return util.InternalError(c, "failed to get bot")
	}

	config, err := bot.GetOpenClawConfig()
	if err != nil {
		return util.InternalError(c, "failed to get bot config")
	}

	if config.Models == nil || config.Models.Providers == nil {
		return util.NotFound(c, "provider not found")
	}

	provider, exists := config.Models.Providers[providerName]
	if !exists {
		return util.NotFound(c, "provider not found")
	}

	return util.Success(c, provider)
}

// UpdateModelProvider updates a model provider configuration
// PUT /bots/:id/config/models/:provider
func UpdateModelProvider(c echo.Context) error {
	id := c.Param("id")
	providerName := c.Param("provider")
	if id == "" {
		return util.BadRequest(c, "id is required")
	}
	if providerName == "" {
		return util.BadRequest(c, "provider name is required")
	}

	var req ProviderRequest
	if err := c.Bind(&req); err != nil {
		return util.BadRequest(c, "invalid request body")
	}

	bot, err := model.GetBotByID(id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return util.NotFound(c, "bot not found")
		}
		return util.InternalError(c, "failed to get bot")
	}

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

	// Update provider
	config.Models.Providers[providerName] = &model.ProviderConfig{
		BaseURL: req.BaseURL,
		APIKey:  req.APIKey,
		Auth:    req.Auth,
		API:     req.API,
		Models:  req.Models,
	}

	// Save to database
	if err := bot.SetOpenClawConfig(config); err != nil {
		return util.InternalError(c, "failed to set config")
	}
	if err := model.UpdateBot(bot); err != nil {
		return util.InternalError(c, "failed to update bot")
	}

	// Sync to pod if bot is running
	if bot.Status == model.BotStatusRunning {
		go func() {
			ctx := context.Background()
			if err := k8s.SyncConfigToPod(ctx, bot.ID); err != nil {
				c.Logger().Errorf("failed to sync config to pod: %v", err)
			}
		}()
	}

	return util.Success(c, config.Models.Providers[providerName])
}

// DeleteModelProvider removes a model provider
// DELETE /bots/:id/config/models/:provider
func DeleteModelProvider(c echo.Context) error {
	id := c.Param("id")
	providerName := c.Param("provider")
	if id == "" {
		return util.BadRequest(c, "id is required")
	}
	if providerName == "" {
		return util.BadRequest(c, "provider name is required")
	}

	bot, err := model.GetBotByID(id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return util.NotFound(c, "bot not found")
		}
		return util.InternalError(c, "failed to get bot")
	}

	config, err := bot.GetOpenClawConfig()
	if err != nil {
		return util.InternalError(c, "failed to get bot config")
	}

	if config.Models == nil || config.Models.Providers == nil {
		return util.NotFound(c, "provider not found")
	}

	if _, exists := config.Models.Providers[providerName]; !exists {
		return util.NotFound(c, "provider not found")
	}

	// Delete provider
	delete(config.Models.Providers, providerName)

	// Save to database
	if err := bot.SetOpenClawConfig(config); err != nil {
		return util.InternalError(c, "failed to set config")
	}
	if err := model.UpdateBot(bot); err != nil {
		return util.InternalError(c, "failed to update bot")
	}

	// Sync to pod if bot is running
	if bot.Status == model.BotStatusRunning {
		go func() {
			ctx := context.Background()
			if err := k8s.SyncConfigToPod(ctx, bot.ID); err != nil {
				c.Logger().Errorf("failed to sync config to pod: %v", err)
			}
		}()
	}

	return util.Success(c, nil)
}

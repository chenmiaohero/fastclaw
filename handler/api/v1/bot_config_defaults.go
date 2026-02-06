package v1

import (
	"context"

	"github.com/labstack/echo/v4"
	"github.com/workany-ai/clawork/middleware"
	"github.com/workany-ai/clawork/model"
	"github.com/workany-ai/clawork/service/k8s"
	"github.com/workany-ai/clawork/util"
)

// AgentDefaultsRequest represents a request to set agent defaults
type AgentDefaultsRequest struct {
	PrimaryModel string `json:"primary_model,omitempty"` // e.g., "anthropic/claude-sonnet-4-20250514"
}

// GetAgentDefaults returns the agent default settings
// GET /bots/:id/config/defaults
func GetAgentDefaults(c echo.Context) error {
	bot := middleware.GetBotFromContext(c)
	if bot == nil {
		return util.Forbidden(c, "not authorized")
	}

	config, err := bot.GetOpenClawConfig()
	if err != nil {
		return util.InternalError(c, "failed to get bot config")
	}

	// Extract agent defaults
	result := map[string]interface{}{}
	if config.Agents != nil && config.Agents.Defaults != nil {
		if config.Agents.Defaults.Model != nil {
			result["primary_model"] = config.Agents.Defaults.Model.Primary
		}
		if config.Agents.Defaults.MaxConcurrent > 0 {
			result["max_concurrent"] = config.Agents.Defaults.MaxConcurrent
		}
	}

	return util.Success(c, result)
}

// SetAgentDefaults sets the agent default settings
// PUT /bots/:id/config/defaults
func SetAgentDefaults(c echo.Context) error {
	bot := middleware.GetBotFromContext(c)
	if bot == nil {
		return util.Forbidden(c, "not authorized")
	}

	var req AgentDefaultsRequest
	if err := c.Bind(&req); err != nil {
		return util.BadRequest(c, "invalid request body")
	}

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
	config.Agents.Defaults.Model.Primary = req.PrimaryModel

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

	return util.Success(c, map[string]string{
		"primary_model": req.PrimaryModel,
	})
}

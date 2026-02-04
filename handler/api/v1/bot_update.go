package v1

import (
	"context"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/workany-ai/clawork/model"
	"github.com/workany-ai/clawork/service/k8s"
	"github.com/workany-ai/clawork/util"
	"gorm.io/gorm"
)

type UpdateBotRequest struct {
	Name      string           `json:"name,omitempty"`
	Slug      string           `json:"slug,omitempty"`
	Password  string           `json:"password,omitempty"` // Optional: update gateway password
	Config    *model.BotConfig `json:"config,omitempty"`
	ExpiresAt *time.Time       `json:"expires_at,omitempty"` // Update expiration time (for renewal)
}

func UpdateBot(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return util.BadRequest(c, "id is required")
	}

	var req UpdateBotRequest
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

	if req.Name != "" {
		bot.Name = req.Name
	}

	if req.Slug != "" {
		if !isValidSlug(req.Slug) {
			return util.BadRequest(c, "slug must be 1-50 characters, lowercase letters, numbers, and hyphens only")
		}
		// Check if slug is already taken by another bot
		existing, _ := model.GetBotBySlug(req.Slug)
		if existing != nil && existing.ID != bot.ID {
			return util.BadRequest(c, "slug is already taken")
		}
		bot.Slug = req.Slug
	}

	// Update password if provided
	if req.Password != "" {
		if len(req.Password) < 4 {
			return util.BadRequest(c, "password must be at least 4 characters")
		}
		bot.Password = req.Password
	}

	// Update config if provided
	if req.Config != nil {
		if err := bot.SetConfig(req.Config); err != nil {
			return util.InternalError(c, "failed to set config")
		}
	}

	// Update expiration time (for renewal)
	if req.ExpiresAt != nil {
		bot.ExpiresAt = req.ExpiresAt
	}

	if err := model.UpdateBot(bot); err != nil {
		return util.InternalError(c, "failed to update bot")
	}

	// If bot is running and config/password was updated, sync to pod
	if (req.Config != nil || req.Password != "") && bot.Status == model.BotStatusRunning {
		// Get config for k8s
		botConfig, _ := bot.GetConfig()
		go func() {
			ctx := context.Background()
			k8sConfig := &k8s.BotConfig{
				Password: bot.Password, // Use bot.Password (from DB field)
			}
			if botConfig != nil {
				k8sConfig.Provider = botConfig.Provider
				k8sConfig.Model = botConfig.Model
				k8sConfig.APIKey = botConfig.APIKey
				k8sConfig.BaseURL = botConfig.BaseURL
				k8sConfig.Auth = botConfig.Auth
				k8sConfig.API = botConfig.API
			}

			// If password was changed, restart pod to apply new auth config
			if req.Password != "" {
				// Update deployment triggers pod restart
				if err := k8s.UpdateDeploymentConfig(ctx, bot.ID, bot.AccessToken, k8sConfig); err != nil {
					c.Logger().Errorf("failed to restart deployment: %v", err)
					return
				}
			}

			// Write config file to pod (force set default model on update)
			if err := k8s.WriteConfigToBot(ctx, bot.ID, k8sConfig, true); err != nil {
				c.Logger().Errorf("failed to sync config to bot: %v", err)
			}
		}()
	}

	return util.Success(c, bot)
}

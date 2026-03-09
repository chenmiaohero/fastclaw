package v1

import (
	"context"

	"github.com/clawhost/clawhost/middleware"
	"github.com/clawhost/clawhost/model"
	"github.com/clawhost/clawhost/service/k8s"
	"github.com/clawhost/clawhost/util"
	"github.com/labstack/echo/v4"
)

func StartBot(c echo.Context) error {
	bot := middleware.GetBotFromContext(c)
	if bot == nil {
		return util.Forbidden(c, "not authorized")
	}

	if bot.Status == model.BotStatusRunning {
		return util.BadRequest(c, "bot is already running")
	}

	ctx := context.Background()

	// Get bot config for deployment
	openclawConfig, _ := bot.GetOpenClawConfig()
	k8sConfig := convertToK8sConfig(bot, openclawConfig)

	// Create K8s deployment
	if err := k8s.CreateDeployment(ctx, bot.ID, bot.UserID, bot.AccessToken, k8sConfig); err != nil {
		return util.InternalError(c, "failed to create deployment: "+err.Error())
	}

	// Create K8s service
	endpoint, err := k8s.CreateService(ctx, bot.ID, bot.UserID)
	if err != nil {
		// Rollback deployment
		k8s.DeleteDeployment(ctx, bot.ID)
		return util.InternalError(c, "failed to create service: "+err.Error())
	}

	// Update bot status
	if err := model.UpdateBotStatus(bot.ID, model.BotStatusRunning, endpoint); err != nil {
		return util.InternalError(c, "failed to update bot status")
	}

	// Write config file to pod (async, don't block the response)
	// Config is required for token auth
	if k8sConfig.AccessToken != "" {
		go func() {
			// On start, only set default model if user hasn't configured one
			if err := k8s.WriteConfigToBot(context.Background(), bot.ID, k8sConfig, false); err != nil {
				// Log error but don't fail the request
				c.Logger().Errorf("failed to write config to bot: %v", err)
			}
		}()
	}

	bot.Status = model.BotStatusRunning
	bot.Endpoint = endpoint

	return util.Success(c, bot)
}

package v1

import (
	"context"

	"github.com/labstack/echo/v4"
	"github.com/workany-ai/clawork/model"
	"github.com/workany-ai/clawork/service/k8s"
	"github.com/workany-ai/clawork/util"
	"gorm.io/gorm"
)

func StartBot(c echo.Context) error {
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

	if bot.Status == model.BotStatusRunning {
		return util.BadRequest(c, "bot is already running")
	}

	ctx := context.Background()

	// Get bot config for deployment
	var k8sConfig *k8s.BotConfig
	if botConfig, err := bot.GetConfig(); err == nil && botConfig != nil {
		k8sConfig = &k8s.BotConfig{
			Model:   botConfig.Model,
			APIKey:  botConfig.APIKey,
			BaseURL: botConfig.BaseURL,
		}
	}

	// Create K8s deployment
	if err := k8s.CreateDeployment(ctx, bot.ID, bot.UserID, k8sConfig); err != nil {
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

	// Always start auto-approve immediately so pairing works right away
	k8s.StartAutoApprove(bot.ID)

	// Write config file to pod (async, don't block the response)
	if k8sConfig != nil && k8sConfig.APIKey != "" {
		go func() {
			if err := k8s.WriteConfigToBot(context.Background(), bot.ID, k8sConfig); err != nil {
				// Log error but don't fail the request
				c.Logger().Errorf("failed to write config to bot: %v", err)
			}
		}()
	}

	bot.Status = model.BotStatusRunning
	bot.Endpoint = endpoint

	return util.Success(c, bot)
}

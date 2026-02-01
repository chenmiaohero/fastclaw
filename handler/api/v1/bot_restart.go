package v1

import (
	"context"

	"github.com/labstack/echo/v4"
	"github.com/workany-ai/clawork/model"
	"github.com/workany-ai/clawork/service/k8s"
	"github.com/workany-ai/clawork/util"
	"gorm.io/gorm"
)

func RestartBot(c echo.Context) error {
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

	if bot.Status != model.BotStatusRunning {
		return util.BadRequest(c, "bot is not running")
	}

	ctx := context.Background()

	// Get bot config
	var k8sConfig *k8s.BotConfig
	if botConfig, err := bot.GetConfig(); err == nil && botConfig != nil {
		k8sConfig = &k8s.BotConfig{
			Model:       botConfig.Model,
			APIKey:      botConfig.APIKey,
			BaseURL:     botConfig.BaseURL,
			AccessToken: bot.AccessToken,
		}
	} else {
		k8sConfig = &k8s.BotConfig{
			AccessToken: bot.AccessToken,
		}
	}

	// Update deployment config and trigger rolling update (zero downtime)
	if err := k8s.UpdateDeploymentConfig(ctx, bot.ID, k8sConfig); err != nil {
		return util.InternalError(c, "failed to update deployment: "+err.Error())
	}

	// Write config file to pod (async, after new pod is ready)
	if k8sConfig != nil && k8sConfig.APIKey != "" {
		go func() {
			if err := k8s.WriteConfigToBot(context.Background(), bot.ID, k8sConfig); err != nil {
				c.Logger().Errorf("failed to write config to bot: %v", err)
			}
		}()
	}

	return util.Success(c, bot)
}

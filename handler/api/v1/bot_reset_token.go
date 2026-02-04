package v1

import (
	"context"

	"github.com/labstack/echo/v4"
	"github.com/workany-ai/clawork/model"
	"github.com/workany-ai/clawork/service/k8s"
	"github.com/workany-ai/clawork/util"
	"gorm.io/gorm"
)

func ResetBotToken(c echo.Context) error {
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

	// Reset the access token
	newToken, err := model.ResetBotAccessToken(id)
	if err != nil {
		return util.InternalError(c, "failed to reset access token")
	}

	// If bot is running, restart it to apply new token
	if bot.Status == model.BotStatusRunning {
		ctx := context.Background()
		if err := k8s.RestartDeployment(ctx, bot.ID); err != nil {
			c.Logger().Warnf("failed to restart deployment after token reset: %v", err)
		}
	}

	return util.Success(c, map[string]interface{}{
		"id":           bot.ID,
		"access_token": newToken,
		"access_url":   buildAccessURL(bot.Slug),
		"message":      "access token has been reset",
	})
}

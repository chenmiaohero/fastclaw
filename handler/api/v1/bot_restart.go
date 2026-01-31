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

	// Restart deployment
	if err := k8s.RestartDeployment(ctx, bot.ID); err != nil {
		return util.InternalError(c, "failed to restart deployment: "+err.Error())
	}

	return util.Success(c, bot)
}

package v1

import (
	"context"

	"github.com/labstack/echo/v4"
	"github.com/workany-ai/clawork/model"
	"github.com/workany-ai/clawork/service/k8s"
	"github.com/workany-ai/clawork/util"
	"gorm.io/gorm"
)

func StopBot(c echo.Context) error {
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

	// Delete K8s deployment (this keeps the PVC data)
	if err := k8s.DeleteDeployment(ctx, bot.ID); err != nil {
		return util.InternalError(c, "failed to delete deployment: "+err.Error())
	}

	// Delete K8s service
	if err := k8s.DeleteService(ctx, bot.ID); err != nil {
		return util.InternalError(c, "failed to delete service: "+err.Error())
	}

	// Update bot status
	if err := model.UpdateBotStatus(bot.ID, model.BotStatusStopped, ""); err != nil {
		return util.InternalError(c, "failed to update bot status")
	}

	bot.Status = model.BotStatusStopped
	bot.Endpoint = ""

	return util.Success(c, bot)
}

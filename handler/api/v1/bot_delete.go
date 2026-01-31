package v1

import (
	"context"

	"github.com/labstack/echo/v4"
	"github.com/workany-ai/clawork/model"
	"github.com/workany-ai/clawork/service/k8s"
	"github.com/workany-ai/clawork/util"
	"gorm.io/gorm"
)

func DeleteBot(c echo.Context) error {
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

	ctx := context.Background()

	// Delete K8s resources if running
	if bot.Status == model.BotStatusRunning {
		if err := k8s.DeleteDeployment(ctx, id); err != nil {
			return util.InternalError(c, "failed to delete deployment")
		}
		if err := k8s.DeleteService(ctx, id); err != nil {
			return util.InternalError(c, "failed to delete service")
		}
	}

	// Delete from database
	if err := model.DeleteBot(id); err != nil {
		return util.InternalError(c, "failed to delete bot")
	}

	// TODO: Clean up NAS data directory

	return util.Success(c, map[string]string{"message": "bot deleted"})
}

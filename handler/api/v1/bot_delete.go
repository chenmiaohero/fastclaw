package v1

import (
	"context"

	"github.com/clawhost/clawhost/middleware"
	"github.com/clawhost/clawhost/model"
	"github.com/clawhost/clawhost/service/k8s"
	"github.com/clawhost/clawhost/util"
	"github.com/labstack/echo/v4"
)

func DeleteBot(c echo.Context) error {
	bot := middleware.GetBotFromContext(c)
	if bot == nil {
		return util.Forbidden(c, "not authorized")
	}

	ctx := context.Background()

	// Delete K8s resources if running
	if bot.Status == model.BotStatusRunning {
		if err := k8s.DeleteDeployment(ctx, bot.ID); err != nil {
			return util.InternalError(c, "failed to delete deployment")
		}
		if err := k8s.DeleteService(ctx, bot.ID); err != nil {
			return util.InternalError(c, "failed to delete service")
		}
	}

	// Delete from database
	if err := model.DeleteBot(bot.ID); err != nil {
		return util.InternalError(c, "failed to delete bot")
	}

	// TODO: Clean up NAS data directory

	return util.Success(c, map[string]string{"message": "bot deleted"})
}

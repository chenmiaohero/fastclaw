package v1

import (
	"context"

	"github.com/labstack/echo/v4"
	"github.com/workany-ai/clawork/middleware"
	"github.com/workany-ai/clawork/model"
	"github.com/workany-ai/clawork/service/k8s"
	"github.com/workany-ai/clawork/util"
)

func RestartBot(c echo.Context) error {
	bot := middleware.GetBotFromContext(c)
	if bot == nil {
		return util.Forbidden(c, "not authorized")
	}

	if bot.Status != model.BotStatusRunning {
		return util.BadRequest(c, "bot is not running")
	}

	ctx := context.Background()

	// Restart deployment (triggers rolling update)
	if err := k8s.RestartDeployment(ctx, bot.ID); err != nil {
		return util.InternalError(c, "failed to restart deployment: "+err.Error())
	}

	// Sync config to pod after restart
	go func() {
		if err := k8s.SyncConfigToPod(context.Background(), bot.ID); err != nil {
			c.Logger().Errorf("failed to sync config to bot: %v", err)
		}
	}()

	return util.Success(c, bot)
}

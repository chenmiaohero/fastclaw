package v1

import (
	"context"

	"github.com/labstack/echo/v4"
	"github.com/workany-ai/clawork/middleware"
	"github.com/workany-ai/clawork/model"
	"github.com/workany-ai/clawork/service/k8s"
	"github.com/workany-ai/clawork/util"
)

// GetBotResponse includes bot info and deployment status
type GetBotResponse struct {
	*model.Bot
	DeploymentStatus *k8s.DeploymentStatusInfo `json:"deployment_status,omitempty"`
}

func GetBot(c echo.Context) error {
	bot := middleware.GetBotFromContext(c)
	if bot == nil {
		return util.Forbidden(c, "not authorized")
	}

	ctx := context.Background()
	response := &GetBotResponse{Bot: bot}

	// If bot is running, get deployment status and sync config
	if bot.Status == model.BotStatusRunning {
		// Get deployment status
		if statusInfo, err := k8s.GetDeploymentStatusInfo(ctx, bot.ID); err == nil {
			response.DeploymentStatus = statusInfo
		}

		// Only sync config if deployment is ready (not during updates)
		if response.DeploymentStatus != nil && response.DeploymentStatus.Status == "ready" {
			if err := k8s.SyncConfigToDatabase(ctx, bot.ID); err != nil {
				c.Logger().Warnf("failed to sync config from pod: %v", err)
			} else {
				// Reload bot to get updated config
				if updatedBot, err := model.GetBotByID(bot.ID); err == nil {
					response.Bot = updatedBot
				}
			}
		}
	}

	return util.Success(c, response)
}

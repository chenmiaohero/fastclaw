package v1

import (
	"context"

	"github.com/labstack/echo/v4"
	"github.com/workany-ai/clawork/model"
	"github.com/workany-ai/clawork/service/k8s"
	"github.com/workany-ai/clawork/util"
	"gorm.io/gorm"
)

type BotStatusResponse struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Status   model.BotStatus   `json:"status"`
	Ready    bool              `json:"ready"`
	Endpoint string            `json:"endpoint,omitempty"`
}

func GetBotStatus(c echo.Context) error {
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

	response := BotStatusResponse{
		ID:       bot.ID,
		Name:     bot.Name,
		Status:   bot.Status,
		Endpoint: bot.Endpoint,
	}

	// Check actual K8s status if bot is supposed to be running
	if bot.Status == model.BotStatusRunning {
		ctx := context.Background()
		ready, err := k8s.GetDeploymentStatus(ctx, bot.ID)
		if err != nil {
			response.Ready = false
		} else {
			response.Ready = ready
		}
	}

	return util.Success(c, response)
}

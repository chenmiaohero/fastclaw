package v1

import (
	"context"

	"github.com/labstack/echo/v4"
	"github.com/workany-ai/clawork/model"
	"github.com/workany-ai/clawork/service/k8s"
	"github.com/workany-ai/clawork/util"
	"gorm.io/gorm"
)

type UpdateBotRequest struct {
	Name   string           `json:"name,omitempty"`
	Config *model.BotConfig `json:"config,omitempty"`
}

func UpdateBot(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return util.BadRequest(c, "id is required")
	}

	var req UpdateBotRequest
	if err := c.Bind(&req); err != nil {
		return util.BadRequest(c, "invalid request body")
	}

	bot, err := model.GetBotByID(id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return util.NotFound(c, "bot not found")
		}
		return util.InternalError(c, "failed to get bot")
	}

	if req.Name != "" {
		bot.Name = req.Name
	}

	if req.Config != nil {
		if err := bot.SetConfig(req.Config); err != nil {
			return util.InternalError(c, "failed to set config")
		}
	}

	if err := model.UpdateBot(bot); err != nil {
		return util.InternalError(c, "failed to update bot")
	}

	// If bot is running and config was updated, sync config to pod
	if req.Config != nil && bot.Status == model.BotStatusRunning {
		go func() {
			k8sConfig := &k8s.BotConfig{
				Model:   req.Config.Model,
				APIKey:  req.Config.APIKey,
				BaseURL: req.Config.BaseURL,
			}
			if err := k8s.WriteConfigToBot(context.Background(), bot.ID, k8sConfig); err != nil {
				c.Logger().Errorf("failed to sync config to bot: %v", err)
			}
		}()
	}

	return util.Success(c, bot)
}

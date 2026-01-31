package v1

import (
	"github.com/labstack/echo/v4"
	"github.com/workany-ai/clawork/model"
	"github.com/workany-ai/clawork/util"
)

type CreateBotRequest struct {
	UserID string           `json:"user_id" validate:"required"`
	Name   string           `json:"name" validate:"required"`
	Config *model.BotConfig `json:"config,omitempty"`
}

func CreateBot(c echo.Context) error {
	var req CreateBotRequest
	if err := c.Bind(&req); err != nil {
		return util.BadRequest(c, "invalid request body")
	}

	if req.UserID == "" {
		return util.BadRequest(c, "user_id is required")
	}
	if req.Name == "" {
		return util.BadRequest(c, "name is required")
	}

	// Check if bot with same name exists for this user
	existing, _ := model.GetBotByUserAndName(req.UserID, req.Name)
	if existing != nil {
		return util.BadRequest(c, "bot with this name already exists")
	}

	bot := &model.Bot{
		UserID: req.UserID,
		Name:   req.Name,
		Status: model.BotStatusCreated,
	}

	if req.Config != nil {
		if err := bot.SetConfig(req.Config); err != nil {
			return util.InternalError(c, "failed to set config")
		}
	}

	if err := model.CreateBot(bot); err != nil {
		return util.InternalError(c, "failed to create bot")
	}

	return util.Success(c, bot)
}

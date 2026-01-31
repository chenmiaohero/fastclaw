package v1

import (
	"github.com/labstack/echo/v4"
	"github.com/workany-ai/clawork/model"
	"github.com/workany-ai/clawork/util"
)

func ListBots(c echo.Context) error {
	userID := c.QueryParam("user_id")
	if userID == "" {
		return util.BadRequest(c, "user_id is required")
	}

	bots, err := model.ListBotsByUserID(userID)
	if err != nil {
		return util.InternalError(c, "failed to list bots")
	}

	return util.Success(c, bots)
}

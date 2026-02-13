package v1

import (
	"github.com/fastclaw-ai/fastclaw/middleware"
	"github.com/fastclaw-ai/fastclaw/model"
	"github.com/fastclaw-ai/fastclaw/util"
	"github.com/labstack/echo/v4"
)

func ListBots(c echo.Context) error {
	userID := c.QueryParam("user_id")
	if userID == "" {
		return util.BadRequest(c, "user_id is required")
	}

	// Get app_id from authenticated app context
	var appID string
	if app := middleware.GetAppFromContext(c); app != nil {
		appID = app.ID
	}

	bots, err := model.ListBotsByAppAndUser(appID, userID)
	if err != nil {
		return util.InternalError(c, "failed to list bots")
	}

	return util.Success(c, bots)
}

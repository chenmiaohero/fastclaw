package v1

import (
	"context"

	"github.com/labstack/echo/v4"
	"github.com/workany-ai/clawork/model"
	"github.com/workany-ai/clawork/service/k8s"
	"github.com/workany-ai/clawork/util"
	"gorm.io/gorm"
)

type AddChannelRequest struct {
	Channel string `json:"channel"` // telegram, discord, slack, whatsapp, etc.
	Token   string `json:"token"`   // Bot token
	// Slack specific
	BotToken string `json:"bot_token,omitempty"` // xoxb-...
	AppToken string `json:"app_token,omitempty"` // xapp-...
}

// AddChannel adds an IM channel to a bot
// POST /bots/:id/channels
func AddChannel(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return util.BadRequest(c, "id is required")
	}

	var req AddChannelRequest
	if err := c.Bind(&req); err != nil {
		return util.BadRequest(c, "invalid request body")
	}

	if req.Channel == "" {
		return util.BadRequest(c, "channel is required")
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

	// Add channel to the running pod
	if err := k8s.AddChannelToBot(context.Background(), bot.ID, bot.AccessToken, req.Channel, req.Token, req.BotToken, req.AppToken); err != nil {
		return util.InternalError(c, "failed to add channel: "+err.Error())
	}

	return util.Success(c, map[string]string{
		"message": "channel added successfully",
		"channel": req.Channel,
	})
}

// ListChannels lists all channels for a bot
// GET /bots/:id/channels
func ListChannels(c echo.Context) error {
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

	// Get channels from the running pod
	channels, err := k8s.ListBotChannels(context.Background(), bot.ID, bot.AccessToken)
	if err != nil {
		return util.InternalError(c, "failed to list channels: "+err.Error())
	}

	return util.Success(c, channels)
}

// RemoveChannel removes an IM channel from a bot
// DELETE /bots/:id/channels/:channel
func RemoveChannel(c echo.Context) error {
	id := c.Param("id")
	channel := c.Param("channel")

	if id == "" {
		return util.BadRequest(c, "id is required")
	}
	if channel == "" {
		return util.BadRequest(c, "channel is required")
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

	// Remove channel from the running pod
	if err := k8s.RemoveChannelFromBot(context.Background(), bot.ID, bot.AccessToken, channel); err != nil {
		return util.InternalError(c, "failed to remove channel: "+err.Error())
	}

	return util.Success(c, map[string]string{
		"message": "channel removed successfully",
	})
}

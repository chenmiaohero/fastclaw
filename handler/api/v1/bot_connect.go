package v1

import (
	"context"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/spf13/viper"
	"github.com/workany-ai/clawork/middleware"
	"github.com/workany-ai/clawork/model"
	"github.com/workany-ai/clawork/service/k8s"
	"github.com/workany-ai/clawork/util"
)

type BotConnectResponse struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Status      model.BotStatus `json:"status"`
	Ready       bool            `json:"ready"`
	Token       string          `json:"token,omitempty"`
	Endpoint    string          `json:"endpoint,omitempty"`
	WsURL       string          `json:"ws_url,omitempty"`
	WebChatURL  string          `json:"webchat_url,omitempty"`
}

func GetBotConnect(c echo.Context) error {
	bot := middleware.GetBotFromContext(c)
	if bot == nil {
		return util.Forbidden(c, "not authorized")
	}

	response := BotConnectResponse{
		ID:     bot.ID,
		Name:   bot.Name,
		Status: bot.Status,
		Token:  bot.ID, // Token is bot ID
	}

	if bot.Status != model.BotStatusRunning {
		return util.Success(c, response)
	}

	ctx := context.Background()

	// Check if deployment is ready
	ready, err := k8s.GetDeploymentStatus(ctx, bot.ID)
	if err != nil {
		return util.InternalError(c, "failed to get deployment status")
	}
	response.Ready = ready

	// Get service endpoint (internal)
	endpoint, err := k8s.GetServiceEndpoint(ctx, bot.ID)
	if err != nil {
		return util.InternalError(c, "failed to get service endpoint")
	}
	response.Endpoint = endpoint

	// Build external URL based on domain template
	if ready {
		serviceName := k8s.GetServiceName(bot.ID)
		namespace := k8s.GetNamespace()

		// Get domain template from config
		domainTemplate := viper.GetString("domain.bot_domain_template")
		if domainTemplate == "" {
			domainTemplate = "http://{service_name}.{namespace}.orb.local:18789"
		}

		// Replace placeholders
		externalURL := strings.ReplaceAll(domainTemplate, "{bot_id}", bot.ID)
		externalURL = strings.ReplaceAll(externalURL, "{service_name}", serviceName)
		externalURL = strings.ReplaceAll(externalURL, "{namespace}", namespace)

		response.WebChatURL = externalURL
		response.WsURL = strings.Replace(externalURL, "http://", "ws://", 1)
		response.WsURL = strings.Replace(response.WsURL, "https://", "wss://", 1)
	}

	return util.Success(c, response)
}

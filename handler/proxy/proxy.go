package proxy

import (
	"context"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/workany-ai/clawork/model"
	"github.com/workany-ai/clawork/service/k8s"
	"github.com/workany-ai/clawork/util"
	"gorm.io/gorm"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for development
	},
}

// ProxyToBot proxies requests to the OpenClaw bot
// Path format: /proxy/{bot_id_or_slug}/*
func ProxyToBot(c echo.Context) error {
	botIdentifier := c.Param("bot_id")
	if botIdentifier == "" {
		return util.BadRequest(c, "bot_id or slug is required")
	}

	// Get bot info - try by ID first, then by slug
	bot, err := model.GetBotByID(botIdentifier)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// Try by slug
			bot, err = model.GetBotBySlug(botIdentifier)
			if err != nil {
				if err == gorm.ErrRecordNotFound {
					return util.NotFound(c, "bot not found")
				}
				return util.InternalError(c, "failed to get bot")
			}
		} else {
			return util.InternalError(c, "failed to get bot")
		}
	}

	// Token is no longer auto-injected - user must provide correct token in URL
	// This ensures only users with the access token can access the bot

	if bot.Status != model.BotStatusRunning {
		return util.BadRequest(c, "bot is not running")
	}

	// Get target URL from K8s service (uses ClusterIP in local dev mode, DNS in production)
	targetHost, err := k8s.GetServiceEndpoint(context.Background(), bot.ID)
	if err != nil {
		return util.InternalError(c, "failed to get service endpoint")
	}
	if targetHost == "" {
		return util.NotFound(c, "bot service not found")
	}

	// Get the remaining path after /proxy/{bot_id}
	remainingPath := c.Param("*")
	if remainingPath == "" {
		remainingPath = "/"
	} else if !strings.HasPrefix(remainingPath, "/") {
		remainingPath = "/" + remainingPath
	}

	// Check if this is a WebSocket upgrade request
	if isWebSocketRequest(c.Request()) {
		return proxyWebSocket(c, targetHost, remainingPath)
	}

	// Regular HTTP proxy
	targetURL := &url.URL{
		Scheme: "http",
		Host:   targetHost,
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = targetHost
		req.URL.Path = remainingPath
		req.URL.RawQuery = c.QueryString()
	}

	proxy.ServeHTTP(c.Response(), c.Request())
	return nil
}

func isWebSocketRequest(r *http.Request) bool {
	return strings.ToLower(r.Header.Get("Upgrade")) == "websocket"
}

func proxyWebSocket(c echo.Context, targetHost, path string) error {
	// Upgrade client connection
	clientConn, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}
	defer clientConn.Close()

	// Connect to backend WebSocket
	backendURL := url.URL{
		Scheme:   "ws",
		Host:     targetHost,
		Path:     path,
		RawQuery: c.QueryString(),
	}

	// Forward relevant headers to backend
	requestHeader := http.Header{}
	if origin := c.Request().Header.Get("Origin"); origin != "" {
		requestHeader.Set("Origin", origin)
	}
	if protocol := c.Request().Header.Get("Sec-WebSocket-Protocol"); protocol != "" {
		requestHeader.Set("Sec-WebSocket-Protocol", protocol)
	}

	backendConn, _, err := websocket.DefaultDialer.Dial(backendURL.String(), requestHeader)
	if err != nil {
		c.Logger().Errorf("WebSocket dial error: %v, url: %s", err, backendURL.String())
		return err
	}
	defer backendConn.Close()

	// Bidirectional message forwarding
	errCh := make(chan error, 2)

	// Client -> Backend
	go func() {
		for {
			msgType, msg, err := clientConn.ReadMessage()
			if err != nil {
				errCh <- err
				return
			}
			if err := backendConn.WriteMessage(msgType, msg); err != nil {
				errCh <- err
				return
			}
		}
	}()

	// Backend -> Client
	go func() {
		for {
			msgType, msg, err := backendConn.ReadMessage()
			if err != nil {
				errCh <- err
				return
			}
			if err := clientConn.WriteMessage(msgType, msg); err != nil {
				errCh <- err
				return
			}
		}
	}()

	// Wait for either direction to close
	<-errCh
	return nil
}

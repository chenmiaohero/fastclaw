package proxy

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strings"

	"github.com/fastclaw-ai/fastclaw/model"
	"github.com/fastclaw-ai/fastclaw/service/k8s"
	"github.com/fastclaw-ai/fastclaw/util"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// isUUID checks if a string is in UUID format
func isUUID(s string) bool {
	uuidRegex := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	return uuidRegex.MatchString(strings.ToLower(s))
}

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

	// Get bot info - determine if it's an ID (UUID format) or slug (short string)
	var bot *model.Bot
	var err error
	if isUUID(botIdentifier) {
		bot, err = model.GetBotByID(botIdentifier)
	} else {
		bot, err = model.GetBotBySlug(botIdentifier)
	}
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return util.NotFound(c, "bot not found")
		}
		return util.InternalError(c, "failed to get bot")
	}

	// Only auto-approve if the request carries the correct access token
	if token := c.QueryParam("token"); token != "" && token == bot.AccessToken {
		go func() {
			ctx := context.Background()
			if err := k8s.AutoApproveAllPending(ctx, bot.ID, bot.AccessToken); err != nil {
				fmt.Printf("[Proxy] Auto-approve failed for bot %s: %v\n", bot.ID, err)
			}
		}()
	}

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

		// Forward real client IP
		clientIP := c.RealIP()
		req.Header.Set("X-Real-IP", clientIP)
		if xff := req.Header.Get("X-Forwarded-For"); xff != "" {
			req.Header.Set("X-Forwarded-For", xff+", "+clientIP)
		} else {
			req.Header.Set("X-Forwarded-For", clientIP)
		}
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
	// Set Origin to the target host to pass OpenClaw's origin check
	// OpenClaw doesn't support wildcard "*" in allowedOrigins
	requestHeader.Set("Origin", fmt.Sprintf("http://%s", targetHost))
	if protocol := c.Request().Header.Get("Sec-WebSocket-Protocol"); protocol != "" {
		requestHeader.Set("Sec-WebSocket-Protocol", protocol)
	}
	// Forward Authorization header for password auth
	if auth := c.Request().Header.Get("Authorization"); auth != "" {
		requestHeader.Set("Authorization", auth)
	}
	// Forward Cookie header (OpenClaw may use cookie for session)
	if cookie := c.Request().Header.Get("Cookie"); cookie != "" {
		requestHeader.Set("Cookie", cookie)
	}
	// Forward real client IP
	clientIP := c.RealIP()
	requestHeader.Set("X-Real-IP", clientIP)
	if xff := c.Request().Header.Get("X-Forwarded-For"); xff != "" {
		requestHeader.Set("X-Forwarded-For", xff+", "+clientIP)
	} else {
		requestHeader.Set("X-Forwarded-For", clientIP)
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

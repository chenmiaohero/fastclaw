package cmd

import (
	"fmt"
	"log"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	v1 "github.com/workany-ai/clawork/handler/api/v1"
	"github.com/workany-ai/clawork/handler/proxy"
	"github.com/workany-ai/clawork/service/k8s"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the API server",
	Run: func(cmd *cobra.Command, args []string) {
		if err := initConfig(); err != nil {
			log.Fatalf("init config failed: %v", err)
		}

		if err := k8s.InitClient(); err != nil {
			log.Fatalf("init k8s client failed: %v", err)
		}

		startServer()
	},
}

func init() {
	rootCmd.AddCommand(serverCmd)
}

func startServer() {
	e := echo.New()

	// Get domain config
	botDomainSuffix := viper.GetString("domain.bot_domain_suffix")
	if botDomainSuffix == "" {
		botDomainSuffix = "workany.loc"
	}

	// Subdomain routing middleware (must run BEFORE routing with e.Pre)
	// {bot-id}.workany.loc/* -> /bot/{bot-id}/*
	e.Pre(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			host := c.Request().Host
			// Remove port if present
			if idx := strings.Index(host, ":"); idx > 0 {
				host = host[:idx]
			}

			// Check if this is a bot subdomain request
			// e.g., 4f1f101c.workany.loc -> extract 4f1f101c
			if strings.HasSuffix(host, "."+botDomainSuffix) {
				botID := strings.TrimSuffix(host, "."+botDomainSuffix)
				if botID != "" && !strings.Contains(botID, ".") {
					// Rewrite to /proxy/{bot-id}/*
					path := c.Request().URL.Path
					c.Request().URL.Path = "/proxy/" + botID + path
				}
			}
			return next(c)
		}
	})

	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	// API routes: /bot/api/v1/*
	api := e.Group("/bot/api/v1")
	{
		// Bot CRUD
		api.POST("/bots", v1.CreateBot)
		api.GET("/bots", v1.ListBots)
		api.GET("/bots/:id", v1.GetBot)
		api.PUT("/bots/:id", v1.UpdateBot)
		api.DELETE("/bots/:id", v1.DeleteBot)

		// Bot lifecycle
		api.POST("/bots/:id/start", v1.StartBot)
		api.POST("/bots/:id/stop", v1.StopBot)
		api.POST("/bots/:id/restart", v1.RestartBot)
		api.GET("/bots/:id/status", v1.GetBotStatus)
		api.GET("/bots/:id/connect", v1.GetBotConnect)

		// Skills management
		api.GET("/bots/:id/skills", v1.ListSkills)
		api.PUT("/bots/:id/skills/:name", v1.UpdateSkill)
		api.DELETE("/bots/:id/skills/:name", v1.DeleteSkill)

		// Channels management (IM integrations)
		api.POST("/bots/:id/channels", v1.AddChannel)
		api.GET("/bots/:id/channels", v1.ListChannels)
		api.DELETE("/bots/:id/channels/:channel", v1.RemoveChannel)
	}

	// Health check
	e.GET("/health", func(c echo.Context) error {
		return c.JSON(200, map[string]string{"status": "ok"})
	})

	// Bot proxy routes (for {bot_id}.workany.loc/*)
	e.Any("/proxy/:bot_id", proxy.ProxyToBot)
	e.Any("/proxy/:bot_id/*", proxy.ProxyToBot)

	port := viper.GetInt("server.port")
	if port == 0 {
		port = 8080
	}

	log.Printf("Starting server on port %d", port)
	e.Logger.Fatal(e.Start(fmt.Sprintf(":%d", port)))
}

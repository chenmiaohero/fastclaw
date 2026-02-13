package cmd

import (
	"fmt"
	"os"

	"github.com/fastclaw-ai/fastclaw/model"
	"github.com/fastclaw-ai/fastclaw/util"
	"github.com/spf13/cobra"
)

var configFile string

var rootCmd = &cobra.Command{
	Use:   "fastclaw",
	Short: "fastclaw is an OpenClaw hosting service",
	Long:  `fastclaw is a multi-tenant OpenClaw hosting service that deploys OpenClaw instances to Kubernetes clusters.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "config.toml", "config file path")
}

func initConfig() error {
	if err := util.InitConfig(configFile); err != nil {
		return fmt.Errorf("init config failed: %w", err)
	}

	if err := util.InitDB(); err != nil {
		return fmt.Errorf("init db failed: %w", err)
	}

	if err := model.AutoMigrate(); err != nil {
		return fmt.Errorf("auto migrate failed: %w", err)
	}

	return nil
}

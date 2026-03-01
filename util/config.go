package util

import (
	"fmt"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

func InitConfig(filename string) error {
	viper.SetConfigFile(filename)

	if err := viper.ReadInConfig(); err != nil {
		return fmt.Errorf("read config file failed: %w", err)
	}

	// Allow environment variables to override config values.
	// FASTCLAW_DB_PASSWORD -> db.password, FASTCLAW_API_ADMIN_TOKEN -> api.admin_token, etc.
	viper.SetEnvPrefix("FASTCLAW")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	viper.WatchConfig()
	viper.OnConfigChange(func(e fsnotify.Event) {
		fmt.Printf("Config file changed: %s\n", e.Name)
	})

	return nil
}

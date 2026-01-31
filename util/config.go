package util

import (
	"fmt"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

func InitConfig(filename string) error {
	viper.SetConfigFile(filename)

	if err := viper.ReadInConfig(); err != nil {
		return fmt.Errorf("read config file failed: %w", err)
	}

	viper.WatchConfig()
	viper.OnConfigChange(func(e fsnotify.Event) {
		fmt.Printf("Config file changed: %s\n", e.Name)
	})

	return nil
}

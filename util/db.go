package util

import (
	"fmt"
	"sync"
	"time"

	"github.com/spf13/viper"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	db   *gorm.DB
	once sync.Once
)

type DBConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Database string `mapstructure:"database"`
	SSLMode  string `mapstructure:"sslmode"`
	Timezone string `mapstructure:"timezone"`
}

func InitDB() error {
	var initErr error
	once.Do(func() {
		var conf DBConfig
		if err := viper.UnmarshalKey("db", &conf); err != nil {
			initErr = fmt.Errorf("unmarshal db config failed: %w", err)
			return
		}

		if conf.SSLMode == "" {
			conf.SSLMode = "disable"
		}
		if conf.Timezone == "" {
			conf.Timezone = "Asia/Shanghai"
		}

		dsn := fmt.Sprintf(
			"host=%s user=%s password=%s dbname=%s port=%d sslmode=%s TimeZone=%s",
			conf.Host, conf.User, conf.Password, conf.Database, conf.Port, conf.SSLMode, conf.Timezone,
		)

		var err error
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Info),
		})
		if err != nil {
			initErr = fmt.Errorf("open db failed: %w", err)
			return
		}

		sqlDB, err := db.DB()
		if err != nil {
			initErr = fmt.Errorf("get sql db failed: %w", err)
			return
		}

		sqlDB.SetMaxIdleConns(10)
		sqlDB.SetMaxOpenConns(100)
		sqlDB.SetConnMaxLifetime(time.Hour)
	})

	return initErr
}

func GetDB() *gorm.DB {
	return db
}

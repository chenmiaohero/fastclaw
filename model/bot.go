package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/workany-ai/clawork/util"
	"gorm.io/gorm"
)

type BotStatus string

const (
	BotStatusCreated BotStatus = "created"
	BotStatusRunning BotStatus = "running"
	BotStatusStopped BotStatus = "stopped"
	BotStatusError   BotStatus = "error"
)

type Bot struct {
	ID        string          `json:"id" gorm:"primaryKey;type:varchar(36)"`
	UserID    string          `json:"user_id" gorm:"type:varchar(36);index;not null"`
	Name      string          `json:"name" gorm:"type:varchar(255);not null"`
	Status    BotStatus       `json:"status" gorm:"type:varchar(50);default:'created'"`
	Config    json.RawMessage `json:"config" gorm:"type:jsonb"`
	Endpoint  string          `json:"endpoint" gorm:"type:varchar(255)"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

type BotConfig struct {
	Model      string      `json:"model,omitempty"`
	APIKey     string      `json:"api_key,omitempty"`
	BaseURL    string      `json:"base_url,omitempty"` // For MiniMax or other Anthropic-compatible APIs
	AgentsMD   string      `json:"agents_md,omitempty"`
	SoulMD     string      `json:"soul_md,omitempty"`
	ToolsMD    string      `json:"tools_md,omitempty"`
	MCPServers []MCPServer `json:"mcp_servers,omitempty"`
}

type MCPServer struct {
	Name         string            `json:"name"`
	Command      string            `json:"command"`
	Args         []string          `json:"args,omitempty"`
	Env          map[string]string `json:"env,omitempty"`
	ShareProcess bool              `json:"share_process,omitempty"`
}

func (Bot) TableName() string {
	return "bots"
}

func (b *Bot) BeforeCreate(tx *gorm.DB) error {
	if b.ID == "" {
		b.ID = uuid.New().String()
	}
	return nil
}

func (b *Bot) GetConfig() (*BotConfig, error) {
	if b.Config == nil {
		return &BotConfig{}, nil
	}
	var config BotConfig
	if err := json.Unmarshal(b.Config, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func (b *Bot) SetConfig(config *BotConfig) error {
	data, err := json.Marshal(config)
	if err != nil {
		return err
	}
	b.Config = data
	return nil
}

// Database operations

func CreateBot(bot *Bot) error {
	return util.GetDB().Create(bot).Error
}

func GetBotByID(id string) (*Bot, error) {
	var bot Bot
	if err := util.GetDB().Where("id = ?", id).First(&bot).Error; err != nil {
		return nil, err
	}
	return &bot, nil
}

func GetBotByUserAndName(userID, name string) (*Bot, error) {
	var bot Bot
	if err := util.GetDB().Where("user_id = ? AND name = ?", userID, name).First(&bot).Error; err != nil {
		return nil, err
	}
	return &bot, nil
}

func ListBotsByUserID(userID string) ([]*Bot, error) {
	var bots []*Bot
	if err := util.GetDB().Where("user_id = ?", userID).Order("created_at DESC").Find(&bots).Error; err != nil {
		return nil, err
	}
	return bots, nil
}

func UpdateBot(bot *Bot) error {
	return util.GetDB().Save(bot).Error
}

func DeleteBot(id string) error {
	return util.GetDB().Where("id = ?", id).Delete(&Bot{}).Error
}

func UpdateBotStatus(id string, status BotStatus, endpoint string) error {
	updates := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	}
	if endpoint != "" {
		updates["endpoint"] = endpoint
	}
	return util.GetDB().Model(&Bot{}).Where("id = ?", id).Updates(updates).Error
}

// AutoMigrate creates the table if it doesn't exist
func AutoMigrate() error {
	return util.GetDB().AutoMigrate(&Bot{})
}

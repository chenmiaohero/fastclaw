package model

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"strings"
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
	ID          string          `json:"id" gorm:"primaryKey;type:varchar(36)"`
	UserID      string          `json:"user_id" gorm:"type:varchar(36);index;not null"`
	Name        string          `json:"name" gorm:"type:varchar(255);not null"`
	Slug        string          `json:"slug" gorm:"type:varchar(100);uniqueIndex;not null"`
	AccessToken string          `json:"access_token" gorm:"type:varchar(64);not null"`
	Status      BotStatus       `json:"status" gorm:"type:varchar(50);default:'created'"`
	Config      json.RawMessage `json:"config" gorm:"type:jsonb"`
	Endpoint    string          `json:"endpoint" gorm:"type:varchar(255)"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
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
	// Generate slug if not provided (use first 8 chars of ID)
	if b.Slug == "" {
		b.Slug = strings.ReplaceAll(b.ID[:8], "-", "")
	}
	// Always generate a secure access token
	if b.AccessToken == "" {
		b.AccessToken = generateSecureToken(32)
	}
	return nil
}

// generateSecureToken generates a cryptographically secure random token
func generateSecureToken(length int) string {
	bytes := make([]byte, length)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)[:length]
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

func GetBotBySlug(slug string) (*Bot, error) {
	var bot Bot
	if err := util.GetDB().Where("slug = ?", slug).First(&bot).Error; err != nil {
		return nil, err
	}
	return &bot, nil
}

func ResetBotAccessToken(id string) (string, error) {
	newToken := generateSecureToken(32)
	err := util.GetDB().Model(&Bot{}).Where("id = ?", id).Updates(map[string]interface{}{
		"access_token": newToken,
		"updated_at":   time.Now(),
	}).Error
	if err != nil {
		return "", err
	}
	return newToken, nil
}

func UpdateBotSlug(id, slug string) error {
	return util.GetDB().Model(&Bot{}).Where("id = ?", id).Updates(map[string]interface{}{
		"slug":       slug,
		"updated_at": time.Now(),
	}).Error
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
	if err := util.GetDB().AutoMigrate(&Bot{}); err != nil {
		return err
	}
	// Migrate existing bots without slug or access_token
	return migrateExistingBots()
}

// migrateExistingBots generates slug and access_token for existing bots
func migrateExistingBots() error {
	var bots []Bot
	if err := util.GetDB().Where("slug = '' OR slug IS NULL OR access_token = '' OR access_token IS NULL").Find(&bots).Error; err != nil {
		return err
	}

	for _, bot := range bots {
		updates := map[string]interface{}{}
		if bot.Slug == "" {
			updates["slug"] = strings.ReplaceAll(bot.ID[:8], "-", "")
		}
		if bot.AccessToken == "" {
			updates["access_token"] = generateSecureToken(32)
		}
		if len(updates) > 0 {
			if err := util.GetDB().Model(&Bot{}).Where("id = ?", bot.ID).Updates(updates).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

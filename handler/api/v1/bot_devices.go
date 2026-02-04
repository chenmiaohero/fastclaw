package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/workany-ai/clawork/model"
	"github.com/workany-ai/clawork/service/k8s"
	"github.com/workany-ai/clawork/util"
	"gorm.io/gorm"
)

// DeviceInfo represents a device in the pairing list
type DeviceInfo struct {
	RequestID  string `json:"request_id,omitempty"`
	DeviceID   string `json:"device_id"`
	Role       string `json:"role,omitempty"`
	Platform   string `json:"platform,omitempty"`
	ClientID   string `json:"client_id,omitempty"`
	ClientMode string `json:"client_mode,omitempty"`
	IP         string `json:"ip,omitempty"`
	Age        string `json:"age,omitempty"`
	Revoked    bool   `json:"revoked,omitempty"`
	Status     string `json:"status"` // pending, paired, or revoked
}

// openclawDeviceList represents the JSON output from openclaw devices list --json
type openclawDeviceList struct {
	Pending []openclawPendingDevice `json:"pending"`
	Paired  []openclawPairedDevice  `json:"paired"`
}

type openclawPendingDevice struct {
	RequestID  string `json:"requestId"`
	DeviceID   string `json:"deviceId"`
	Role       string `json:"role"`
	Platform   string `json:"platform"`
	ClientID   string `json:"clientId"`
	ClientMode string `json:"clientMode"`
	IP         string `json:"ip"`
	Ts         int64  `json:"ts"` // timestamp in ms
}

type openclawPairedDevice struct {
	DeviceID     string                `json:"deviceId"`
	Role         string                `json:"role"`
	Platform     string                `json:"platform"`
	ClientID     string                `json:"clientId"`
	ClientMode   string                `json:"clientMode"`
	CreatedAtMs  int64                 `json:"createdAtMs"`
	ApprovedAtMs int64                 `json:"approvedAtMs"`
	Tokens       []openclawDeviceToken `json:"tokens"`
}

type openclawDeviceToken struct {
	Role        string `json:"role"`
	RevokedAtMs int64  `json:"revokedAtMs,omitempty"`
}

// formatAge formats a duration as a human-readable age string
func formatAge(ms int64) string {
	if ms == 0 {
		return ""
	}
	d := time.Since(time.UnixMilli(ms))
	if d < time.Minute {
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	} else if d < time.Hour {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	} else if d < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(d.Hours()/24))
}

// ListDevices returns the list of pending and paired devices for a bot
// Query params:
//   - status: filter by status ("pending" or "paired"), default returns all
func ListDevices(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return util.BadRequest(c, "id is required")
	}

	statusFilter := c.QueryParam("status") // "pending", "paired", or empty for all

	bot, err := model.GetBotByID(id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return util.NotFound(c, "bot not found")
		}
		return util.InternalError(c, "failed to get bot")
	}

	if bot.Status != model.BotStatusRunning {
		return util.BadRequest(c, "bot is not running")
	}

	ctx := context.Background()

	// Get pod name
	podName, err := k8s.GetPodName(ctx, bot.ID)
	if err != nil {
		return util.InternalError(c, "failed to get pod: "+err.Error())
	}

	// Execute devices list command with --json flag and token for gateway auth
	output, err := k8s.ExecInPod(ctx, k8s.GetNamespace(), podName, "openclaw",
		[]string{"node", "/app/openclaw.mjs", "devices", "list", "--json", "--token", bot.AccessToken})
	if err != nil {
		return util.InternalError(c, "failed to list devices: "+err.Error())
	}

	// Parse JSON output
	var deviceList openclawDeviceList
	if err := json.Unmarshal([]byte(output), &deviceList); err != nil {
		return util.InternalError(c, "failed to parse devices: "+err.Error())
	}

	// Convert to DeviceInfo
	var devices []DeviceInfo
	for _, d := range deviceList.Pending {
		devices = append(devices, DeviceInfo{
			RequestID:  d.RequestID,
			DeviceID:   d.DeviceID,
			Role:       d.Role,
			Platform:   d.Platform,
			ClientID:   d.ClientID,
			ClientMode: d.ClientMode,
			IP:         d.IP,
			Age:        formatAge(d.Ts),
			Status:     "pending",
		})
	}
	for _, d := range deviceList.Paired {
		// Check if device is revoked (all tokens revoked)
		revoked := false
		if len(d.Tokens) > 0 {
			allRevoked := true
			for _, t := range d.Tokens {
				if t.RevokedAtMs == 0 {
					allRevoked = false
					break
				}
			}
			revoked = allRevoked
		}

		status := "paired"
		if revoked {
			status = "revoked"
		}

		devices = append(devices, DeviceInfo{
			DeviceID:   d.DeviceID,
			Role:       d.Role,
			Platform:   d.Platform,
			ClientID:   d.ClientID,
			ClientMode: d.ClientMode,
			Age:        formatAge(d.ApprovedAtMs),
			Revoked:    revoked,
			Status:     status,
		})
	}

	// Filter by status if specified
	if statusFilter != "" {
		var filtered []DeviceInfo
		for _, d := range devices {
			if d.Status == statusFilter {
				filtered = append(filtered, d)
			}
		}
		devices = filtered
	}

	return util.Success(c, map[string]interface{}{
		"bot_id":  bot.ID,
		"devices": devices,
	})
}

// ApproveDevice approves a pending device pairing request
func ApproveDevice(c echo.Context) error {
	id := c.Param("id")
	requestID := c.Param("request_id")

	if id == "" {
		return util.BadRequest(c, "id is required")
	}
	if requestID == "" {
		return util.BadRequest(c, "request_id is required")
	}

	bot, err := model.GetBotByID(id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return util.NotFound(c, "bot not found")
		}
		return util.InternalError(c, "failed to get bot")
	}

	if bot.Status != model.BotStatusRunning {
		return util.BadRequest(c, "bot is not running")
	}

	ctx := context.Background()

	// Get pod name
	podName, err := k8s.GetPodName(ctx, bot.ID)
	if err != nil {
		return util.InternalError(c, "failed to get pod: "+err.Error())
	}

	// Execute approve command with token for gateway auth
	output, err := k8s.ExecInPod(ctx, k8s.GetNamespace(), podName, "openclaw",
		[]string{"node", "/app/openclaw.mjs", "devices", "approve", requestID, "--token", bot.AccessToken})
	if err != nil {
		return util.InternalError(c, "failed to approve device: "+err.Error())
	}

	return util.Success(c, map[string]interface{}{
		"bot_id":     bot.ID,
		"request_id": requestID,
		"message":    "device approved",
		"output":     strings.TrimSpace(output),
	})
}

// RevokeDevice revokes a paired device
// Query params:
//   - role: the role to revoke (default: "user")
func RevokeDevice(c echo.Context) error {
	id := c.Param("id")
	deviceID := c.Param("device_id")
	role := c.QueryParam("role")
	if role == "" {
		role = "operator" // default role
	}

	if id == "" {
		return util.BadRequest(c, "id is required")
	}
	if deviceID == "" {
		return util.BadRequest(c, "device_id is required")
	}

	bot, err := model.GetBotByID(id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return util.NotFound(c, "bot not found")
		}
		return util.InternalError(c, "failed to get bot")
	}

	if bot.Status != model.BotStatusRunning {
		return util.BadRequest(c, "bot is not running")
	}

	ctx := context.Background()

	// Get pod name
	podName, err := k8s.GetPodName(ctx, bot.ID)
	if err != nil {
		return util.InternalError(c, "failed to get pod: "+err.Error())
	}

	// Execute revoke command with token for gateway auth
	output, err := k8s.ExecInPod(ctx, k8s.GetNamespace(), podName, "openclaw",
		[]string{"node", "/app/openclaw.mjs", "devices", "revoke", "--device", deviceID, "--role", role, "--token", bot.AccessToken})
	if err != nil {
		return util.InternalError(c, "failed to revoke device: "+err.Error())
	}

	return util.Success(c, map[string]interface{}{
		"bot_id":    bot.ID,
		"device_id": deviceID,
		"role":      role,
		"message":   "device revoked",
		"output":    strings.TrimSpace(output),
	})
}


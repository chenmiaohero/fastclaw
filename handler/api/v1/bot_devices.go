package v1

import (
	"context"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/workany-ai/clawork/model"
	"github.com/workany-ai/clawork/service/k8s"
	"github.com/workany-ai/clawork/util"
	"gorm.io/gorm"
)

// DeviceInfo represents a device in the pairing list
type DeviceInfo struct {
	RequestID string `json:"request_id,omitempty"`
	DeviceID  string `json:"device_id"`
	Role      string `json:"role,omitempty"`
	IP        string `json:"ip,omitempty"`
	Age       string `json:"age,omitempty"`
	Status    string `json:"status"` // pending or paired
}

// ListDevices returns the list of pending and paired devices for a bot
func ListDevices(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return util.BadRequest(c, "id is required")
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

	// Execute devices list command
	output, err := k8s.ExecInPod(ctx, k8s.GetNamespace(), podName, "openclaw",
		[]string{"node", "/app/openclaw.mjs", "devices", "list"})
	if err != nil {
		return util.InternalError(c, "failed to list devices: "+err.Error())
	}

	// Parse the output
	devices := parseDeviceList(output)

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

	// Execute approve command
	output, err := k8s.ExecInPod(ctx, k8s.GetNamespace(), podName, "openclaw",
		[]string{"node", "/app/openclaw.mjs", "devices", "approve", requestID})
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
func RevokeDevice(c echo.Context) error {
	id := c.Param("id")
	deviceID := c.Param("device_id")

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

	// Execute revoke command
	output, err := k8s.ExecInPod(ctx, k8s.GetNamespace(), podName, "openclaw",
		[]string{"node", "/app/openclaw.mjs", "devices", "revoke", deviceID})
	if err != nil {
		return util.InternalError(c, "failed to revoke device: "+err.Error())
	}

	return util.Success(c, map[string]interface{}{
		"bot_id":    bot.ID,
		"device_id": deviceID,
		"message":   "device revoked",
		"output":    strings.TrimSpace(output),
	})
}

// parseDeviceList parses the output of `openclaw devices list`
func parseDeviceList(output string) []DeviceInfo {
	var devices []DeviceInfo
	lines := strings.Split(output, "\n")

	inPendingSection := false
	inPairedSection := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.Contains(line, "Pending") {
			inPendingSection = true
			inPairedSection = false
			continue
		}
		if strings.Contains(line, "Paired") {
			inPendingSection = false
			inPairedSection = true
			continue
		}

		// Skip header and separator lines
		if line == "" || strings.HasPrefix(line, "Request") || strings.HasPrefix(line, "Device") ||
		   strings.HasPrefix(line, "─") || strings.HasPrefix(line, "│") && strings.Count(line, "│") < 3 {
			continue
		}

		// Parse table rows (separated by │)
		parts := strings.Split(line, "│")
		if len(parts) < 3 {
			continue
		}

		// Clean up parts
		var cleanParts []string
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				cleanParts = append(cleanParts, p)
			}
		}

		if inPendingSection && len(cleanParts) >= 4 {
			// Pending: Request | Device | Role | IP | Age | Flags
			devices = append(devices, DeviceInfo{
				RequestID: cleanParts[0],
				DeviceID:  cleanParts[1],
				Role:      cleanParts[2],
				IP:        cleanParts[3],
				Age:       safeGet(cleanParts, 4),
				Status:    "pending",
			})
		} else if inPairedSection && len(cleanParts) >= 2 {
			// Paired: Device | Roles | Scopes | Tokens | IP
			devices = append(devices, DeviceInfo{
				DeviceID: cleanParts[0],
				Role:     safeGet(cleanParts, 1),
				IP:       safeGet(cleanParts, 4),
				Status:   "paired",
			})
		}
	}

	return devices
}

func safeGet(arr []string, index int) string {
	if index < len(arr) {
		return arr[index]
	}
	return ""
}

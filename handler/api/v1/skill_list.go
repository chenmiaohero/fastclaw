package v1

import (
	"context"
	"fmt"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/workany-ai/clawork/model"
	"github.com/workany-ai/clawork/service/k8s"
	"github.com/workany-ai/clawork/util"
	"gorm.io/gorm"
)

type SkillInfo struct {
	Name string `json:"name"`
}

func ListSkills(c echo.Context) error {
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
		return util.BadRequest(c, "bot is not running, cannot list skills")
	}

	ctx := context.Background()

	// List skills directory in pod
	skills, err := listSkillsInPod(ctx, bot.ID)
	if err != nil {
		return util.InternalError(c, "failed to list skills: "+err.Error())
	}

	return util.Success(c, skills)
}

func listSkillsInPod(ctx context.Context, botID string) ([]SkillInfo, error) {
	client := k8s.GetClient()
	namespace := k8s.GetNamespace()

	// Get pod name
	deploymentName := k8s.GetDeploymentName(botID)
	pods, err := client.CoreV1().Pods(namespace).List(ctx, k8s.ListOptions(deploymentName))
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	if len(pods.Items) == 0 {
		return []SkillInfo{}, nil
	}

	podName := pods.Items[0].Name

	// Execute ls command in pod
	output, err := k8s.ExecInPod(ctx, namespace, podName, "openclaw", []string{"ls", "-1", "/app/.openclaw/workspace/skills"})
	if err != nil {
		// Directory might not exist yet
		return []SkillInfo{}, nil
	}

	var skills []SkillInfo
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		if line != "" {
			skills = append(skills, SkillInfo{Name: line})
		}
	}

	return skills, nil
}

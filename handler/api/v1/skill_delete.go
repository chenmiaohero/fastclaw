package v1

import (
	"context"
	"fmt"

	"github.com/labstack/echo/v4"
	"github.com/workany-ai/clawork/model"
	"github.com/workany-ai/clawork/service/k8s"
	"github.com/workany-ai/clawork/util"
	"gorm.io/gorm"
)

func DeleteSkill(c echo.Context) error {
	id := c.Param("id")
	name := c.Param("name")

	if id == "" {
		return util.BadRequest(c, "id is required")
	}
	if name == "" {
		return util.BadRequest(c, "skill name is required")
	}

	bot, err := model.GetBotByID(id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return util.NotFound(c, "bot not found")
		}
		return util.InternalError(c, "failed to get bot")
	}

	if bot.Status != model.BotStatusRunning {
		return util.BadRequest(c, "bot is not running, cannot delete skills")
	}

	ctx := context.Background()

	// Delete skill directory in pod
	if err := deleteSkillFromPod(ctx, bot.ID, name); err != nil {
		return util.InternalError(c, "failed to delete skill: "+err.Error())
	}

	return util.Success(c, map[string]string{
		"message": "skill deleted",
		"name":    name,
	})
}

func deleteSkillFromPod(ctx context.Context, botID, skillName string) error {
	client := k8s.GetClient()
	namespace := k8s.GetNamespace()

	// Get pod name
	deploymentName := k8s.GetDeploymentName(botID)
	pods, err := client.CoreV1().Pods(namespace).List(ctx, k8s.ListOptions(deploymentName))
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}

	if len(pods.Items) == 0 {
		return fmt.Errorf("no running pod found")
	}

	podName := pods.Items[0].Name
	skillPath := fmt.Sprintf("/app/.openclaw/workspace/skills/%s", skillName)

	// Remove directory
	_, err = k8s.ExecInPod(ctx, namespace, podName, "openclaw", []string{"rm", "-rf", skillPath})
	if err != nil {
		return fmt.Errorf("failed to delete skill directory: %w", err)
	}

	return nil
}

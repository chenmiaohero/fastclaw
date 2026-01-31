package k8s

import (
	"context"
	"fmt"

	"github.com/spf13/viper"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// getShortID returns first 8 chars of botID to keep K8s names under 63 chars
func getShortID(botID string) string {
	if len(botID) > 8 {
		return botID[:8]
	}
	return botID
}

func GetDeploymentName(botID string) string {
	return fmt.Sprintf("oc-%s", getShortID(botID))
}

func GetServiceName(botID string) string {
	return fmt.Sprintf("oc-%s-svc", getShortID(botID))
}

// BotConfig holds the configuration for a bot
type BotConfig struct {
	Model   string
	APIKey  string
	BaseURL string // For MiniMax or other Anthropic-compatible APIs
}

func CreateDeployment(ctx context.Context, botID, userID string, config *BotConfig) error {
	client := GetClient()
	namespace := GetNamespace()
	deploymentName := GetDeploymentName(botID)

	// Get config values
	image := viper.GetString("openclaw.image")
	if image == "" {
		image = "openclaw/openclaw:latest"
	}
	gatewayPort := viper.GetInt32("openclaw.gateway_port")
	if gatewayPort == 0 {
		gatewayPort = 18789
	}
	pvcName := viper.GetString("storage.pvc_name")
	if pvcName == "" {
		pvcName = "openclaw-shared-data"
	}
	basePath := viper.GetString("storage.base_path")
	if basePath == "" {
		basePath = "/openclaw-data"
	}

	cpuLimit := viper.GetString("openclaw.cpu_limit")
	if cpuLimit == "" {
		cpuLimit = "500m"
	}
	memoryLimit := viper.GetString("openclaw.memory_limit")
	if memoryLimit == "" {
		memoryLimit = "512Mi"
	}
	cpuRequest := viper.GetString("openclaw.cpu_request")
	if cpuRequest == "" {
		cpuRequest = "100m"
	}
	memoryRequest := viper.GetString("openclaw.memory_request")
	if memoryRequest == "" {
		memoryRequest = "128Mi"
	}

	labels := map[string]string{
		"app":     "openclaw",
		"bot-id":  botID,
		"user-id": userID,
	}

	replicas := int32(1)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "openclaw",
							Image: image,
							Ports: []corev1.ContainerPort{
								{
									Name:          "gateway",
									ContainerPort: gatewayPort,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							Command: []string{"node", "/app/openclaw.mjs", "gateway", "--port", fmt.Sprintf("%d", gatewayPort), "--bind", "lan", "--allow-unconfigured", "--dev", "--token", botID},
							Env: func() []corev1.EnvVar {
								envs := []corev1.EnvVar{
									{
										Name:  "OPENCLAW_GATEWAY_TOKEN",
										Value: botID,
									},
								}
								if config != nil {
									if config.APIKey != "" {
										envs = append(envs, corev1.EnvVar{
											Name:  "ANTHROPIC_API_KEY",
											Value: config.APIKey,
										})
									}
									if config.Model != "" {
										envs = append(envs, corev1.EnvVar{
											Name:  "CLAUDE_MODEL",
											Value: config.Model,
										})
									}
									if config.BaseURL != "" {
										envs = append(envs, corev1.EnvVar{
											Name:  "ANTHROPIC_BASE_URL",
											Value: config.BaseURL,
										})
									}
								}
								return envs
							}(),
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "data",
									MountPath: "/home/node/.openclaw",
									SubPath:   botID,
								},
							},
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse(cpuLimit),
									corev1.ResourceMemory: resource.MustParse(memoryLimit),
								},
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse(cpuRequest),
									corev1.ResourceMemory: resource.MustParse(memoryRequest),
								},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.FromInt32(gatewayPort),
									},
								},
								InitialDelaySeconds: 120,
								PeriodSeconds:       30,
								FailureThreshold:    5,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.FromInt32(gatewayPort),
									},
								},
								InitialDelaySeconds: 30,
								PeriodSeconds:       10,
								FailureThreshold:    10,
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "data",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: pvcName,
								},
							},
						},
					},
				},
			},
		},
	}

	_, err := client.AppsV1().Deployments(namespace).Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil {
		if errors.IsAlreadyExists(err) {
			return nil
		}
		return fmt.Errorf("failed to create deployment: %w", err)
	}

	return nil
}

func DeleteDeployment(ctx context.Context, botID string) error {
	client := GetClient()
	namespace := GetNamespace()
	deploymentName := GetDeploymentName(botID)

	err := client.AppsV1().Deployments(namespace).Delete(ctx, deploymentName, metav1.DeleteOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to delete deployment: %w", err)
	}

	return nil
}

func GetDeploymentStatus(ctx context.Context, botID string) (bool, error) {
	client := GetClient()
	namespace := GetNamespace()
	deploymentName := GetDeploymentName(botID)

	deployment, err := client.AppsV1().Deployments(namespace).Get(ctx, deploymentName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to get deployment: %w", err)
	}

	return deployment.Status.ReadyReplicas > 0, nil
}

func RestartDeployment(ctx context.Context, botID string) error {
	client := GetClient()
	namespace := GetNamespace()
	deploymentName := GetDeploymentName(botID)

	// Get current deployment
	deployment, err := client.AppsV1().Deployments(namespace).Get(ctx, deploymentName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get deployment: %w", err)
	}

	// Add/update restart annotation to trigger rollout
	if deployment.Spec.Template.Annotations == nil {
		deployment.Spec.Template.Annotations = make(map[string]string)
	}
	deployment.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = metav1.Now().Format("2006-01-02T15:04:05Z07:00")

	_, err = client.AppsV1().Deployments(namespace).Update(ctx, deployment, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update deployment: %w", err)
	}

	return nil
}


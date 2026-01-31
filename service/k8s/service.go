package k8s

import (
	"context"
	"fmt"

	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func CreateService(ctx context.Context, botID, userID string) (string, error) {
	client := GetClient()
	namespace := GetNamespace()
	serviceName := GetServiceName(botID)

	gatewayPort := viper.GetInt32("openclaw.gateway_port")
	if gatewayPort == 0 {
		gatewayPort = 18789
	}

	labels := map[string]string{
		"app":     "openclaw",
		"bot-id":  botID,
		"user-id": userID,
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports: []corev1.ServicePort{
				{
					Name:       "gateway",
					Port:       gatewayPort,
					TargetPort: intstr.FromInt(int(gatewayPort)),
					Protocol:   corev1.ProtocolTCP,
				},
			},
			Type: corev1.ServiceTypeClusterIP,
		},
	}

	created, err := client.CoreV1().Services(namespace).Create(ctx, service, metav1.CreateOptions{})
	if err != nil {
		if errors.IsAlreadyExists(err) {
			// Get existing service
			existing, getErr := client.CoreV1().Services(namespace).Get(ctx, serviceName, metav1.GetOptions{})
			if getErr != nil {
				return "", fmt.Errorf("failed to get existing service: %w", getErr)
			}
			return fmt.Sprintf("%s.%s.svc.cluster.local:%d", existing.Name, existing.Namespace, gatewayPort), nil
		}
		return "", fmt.Errorf("failed to create service: %w", err)
	}

	return fmt.Sprintf("%s.%s.svc.cluster.local:%d", created.Name, created.Namespace, gatewayPort), nil
}

func DeleteService(ctx context.Context, botID string) error {
	client := GetClient()
	namespace := GetNamespace()
	serviceName := GetServiceName(botID)

	err := client.CoreV1().Services(namespace).Delete(ctx, serviceName, metav1.DeleteOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to delete service: %w", err)
	}

	return nil
}

func GetServiceEndpoint(ctx context.Context, botID string) (string, error) {
	client := GetClient()
	namespace := GetNamespace()
	serviceName := GetServiceName(botID)

	gatewayPort := viper.GetInt32("openclaw.gateway_port")
	if gatewayPort == 0 {
		gatewayPort = 18789
	}

	service, err := client.CoreV1().Services(namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return "", nil
		}
		return "", fmt.Errorf("failed to get service: %w", err)
	}

	// In local dev mode, use ClusterIP directly (OrbStack allows direct access from host)
	// In production, use DNS name for in-cluster communication
	localDev := viper.GetBool("kubernetes.local_dev")
	if localDev && service.Spec.ClusterIP != "" && service.Spec.ClusterIP != "None" {
		return fmt.Sprintf("%s:%d", service.Spec.ClusterIP, gatewayPort), nil
	}

	return fmt.Sprintf("%s.%s.svc.cluster.local:%d", service.Name, service.Namespace, gatewayPort), nil
}

package enhancedlivenessprobemapping

import (
	corev1 "k8s.io/api/core/v1"
)

type Interface interface {
	GetPodNodeName(pod *corev1.Pod) string
}

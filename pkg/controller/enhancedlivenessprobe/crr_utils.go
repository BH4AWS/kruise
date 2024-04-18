package enhancedlivenessprobe

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"

	appsv1alpha1 "github.com/openkruise/kruise/apis/apps/v1alpha1"
)

// generate CRR config by nodePodProbe related with pod
func generateCRRByNodePodProbeConfig(nodePodProbe appsv1alpha1.NodePodProbe, podLivenessFailedContainers podLivenessFailedContainers) (appsv1alpha1.ContainerRecreateRequest, error) {

	ttLSecondsAfterFinished := int32(600)
	activeDeadlineSeconds := int64(1800) // 30min = 60s * 30 = 1800s

	isController := true
	crrTemplate := appsv1alpha1.ContainerRecreateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    podLivenessFailedContainers.Pod.Namespace,
			GenerateName: fmt.Sprintf("%v-%v-", nodePodProbe.Name, flagEliveness),
			Labels: map[string]string{
				"owner":   controllerName,
				"usage":   nodePodProbe.Name, // CRR is related with the nodePodProbe
				podUIDKey: fmt.Sprintf("%v", podLivenessFailedContainers.Pod.UID),
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					Name:       nodePodProbe.Name,
					Kind:       nodePodProbe.Kind,
					Controller: &isController,
				},
			},
		},
		Spec: appsv1alpha1.ContainerRecreateRequestSpec{
			PodName: podLivenessFailedContainers.Pod.Name,
			Strategy: &appsv1alpha1.ContainerRecreateRequestStrategy{
				FailurePolicy: appsv1alpha1.ContainerRecreateRequestFailurePolicyFail,
				ForceRecreate: true,
			},
			ActiveDeadlineSeconds:   &activeDeadlineSeconds,
			TTLSecondsAfterFinished: &ttLSecondsAfterFinished,
		},
	}
	for _, cName := range podLivenessFailedContainers.FailedContainers {
		crrTemplate.Spec.Containers = append(crrTemplate.Spec.Containers, appsv1alpha1.ContainerRecreateRequestContainer{
			Name: cName,
		})
	}
	return crrTemplate, nil
}

func (r *ReconcileEnhancedLivenessProbe) getCRRByNamespacedName(ctx context.Context, crrNamespace, crrName string) (*appsv1alpha1.ContainerRecreateRequest, error) {
	existingCRR := &appsv1alpha1.ContainerRecreateRequest{}
	err := r.Get(ctx, types.NamespacedName{Namespace: crrNamespace, Name: crrName}, existingCRR)
	if err != nil {
		klog.Errorf("Failed to get CRR object: %v/%v, err: %v", crrNamespace, crrName, err)
		return existingCRR, err
	}
	return existingCRR, nil
}

func (r *ReconcileEnhancedLivenessProbe) deleteCRRByNamespacedName(ctx context.Context, crrNamespace, crrName string) error {
	existingCRR, err := r.getCRRByNamespacedName(ctx, crrNamespace, crrName)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		klog.Errorf("Failed to get CRR object: %v/%v, err: %v", crrNamespace, crrName, err)
		return err
	}
	return r.Delete(ctx, existingCRR)
}

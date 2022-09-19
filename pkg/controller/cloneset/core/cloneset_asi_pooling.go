package core

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	appsv1alpha1 "github.com/openkruise/kruise/apis/apps/v1alpha1"
	"github.com/openkruise/kruise/pkg/controller/cloneset/apiinternal"
	clonesetutils "github.com/openkruise/kruise/pkg/controller/cloneset/utils"
	"github.com/openkruise/kruise/pkg/util/inplaceupdate"
	"github.com/openkruise/kruise/pkg/utilasi"
	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog"
	kubecontroller "k8s.io/kubernetes/pkg/controller"
	"k8s.io/utils/integer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (c *asiControl) createWithPooling(replicas int, revision string, poolConfig *appsv1alpha1.PoolConfig) (int, error) {
	if len(poolConfig.Pools) == 0 {
		return 0, nil
	}

	var allAdopted []*v1.Pod
	poolReplicas := getPoolsWithReplicas(replicas, poolConfig)
	for i := range poolConfig.Pools {
		pool := &poolConfig.Pools[i]
		num := poolReplicas[pool.Name]

		adopted, err := c.adoptPodsAndPVCsFromPool(num, revision, pool, poolConfig.PatchTemplate)
		if len(adopted) > 0 {
			allAdopted = append(allAdopted, adopted...)
		}
		if err != nil {
			klog.Errorf("CloneSet %s/%s has failed to adopt pool %v, adopted pods %v, error: %v",
				c.Namespace, c.Name, pool.Name, utilasi.GetPodNames(adopted), err)
			return len(allAdopted), err
		}
		klog.V(5).Infof("CloneSet %s/%s has adopted pool %v with pods %v",
			c.Namespace, c.Name, pool.Name, utilasi.GetPodNames(adopted))
	}

	return len(allAdopted), nil
}

func (c *asiControl) adoptPodsAndPVCsFromPool(replicas int, revision string, pool *appsv1alpha1.PoolTerm, patchTemplate runtime.RawExtension) (adopted []*v1.Pod, retErr error) {
	trueVal := true
	podList := v1.PodList{}
	if err := gClient.List(context.TODO(), &podList, client.InNamespace(c.Namespace), client.MatchingLabels(pool.MatchSelector)); err != nil {
		return nil, err
	}

	defer func() {
		if len(adopted) > 0 {
			c.waitForInformerWatched(adopted[len(adopted)-1])
		}
	}()
	for i := range podList.Items {
		if !kubecontroller.IsPodActive(&podList.Items[i]) {
			continue
		}
		pod := podList.Items[i].DeepCopy()

		newOwner := metav1.OwnerReference{
			APIVersion:         clonesetutils.ControllerKind.GroupVersion().String(),
			Kind:               clonesetutils.ControllerKind.Kind,
			Name:               c.Name,
			UID:                c.UID,
			Controller:         &trueVal,
			BlockOwnerDeletion: &trueVal,
		}

		// 1. 接管 cloneset 配套创建的 pvc
		if err := c.adoptPVCsForPod(pod, newOwner); err != nil {
			return adopted, fmt.Errorf("failed to adopt pvc for Pod %s: %v", pod.Name, err)
		}

		// 2. 替换 owner
		utilasi.ReplaceOwnerRef(pod, newOwner)

		// 3. 更新固定信息
		for k, v := range c.Spec.Selector.MatchLabels {
			pod.Labels[k] = v
		}
		if c.Spec.UpdateStrategy.Type != appsv1alpha1.InPlaceOnlyCloneSetUpdateStrategyType {
			inplaceupdate.InjectReadinessGate(pod)
		}
		pod.Labels[apiinternal.LabelPodUpgradingState] = apiinternal.PodUpgradingExecuting
		pod.Labels[apiinternal.LabelFinalStateUpgrading] = "true"
		pod.Labels[apps.StatefulSetRevisionLabel] = revision
		if pod.Labels[apiinternal.LabelScheduleNodeName] != "" {
			utilasi.SetUpdateSpecHash(pod, revision)
		}

		// 4. 更新 patchTemplate
		if patchTemplate.Raw != nil {
			cloneBytes, _ := json.Marshal(pod)
			modified, err := strategicpatch.StrategicMergePatch(cloneBytes, patchTemplate.Raw, &v1.Pod{})
			if err != nil {
				return adopted, err
			}
			pod = &v1.Pod{}
			if err = json.Unmarshal(modified, pod); err != nil {
				return adopted, err
			}
		}

		// 5. update pod
		if err := gClient.Update(context.TODO(), pod); err != nil {
			if errors.IsConflict(err) {
				klog.Warningf("CloneSet %s/%s adopt pooling pod %s conflict", c.Namespace, c.Name, pod.Name)
				continue
			}
			return adopted, fmt.Errorf("failed to adopt pod %s: %v", pod.Name, err)
		}
		adopted = append(adopted, pod)

		if len(adopted) == replicas {
			break
		}
	}

	return adopted, nil
}

func (c *asiControl) adoptPVCsForPod(pod *v1.Pod, owner metav1.OwnerReference) error {
	for i := range pod.Spec.Volumes {
		vol := &pod.Spec.Volumes[i]
		if vol.PersistentVolumeClaim == nil {
			return nil
		}
		err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			pvc := &v1.PersistentVolumeClaim{}
			err := gClient.Get(context.TODO(), types.NamespacedName{Namespace: pod.Namespace, Name: vol.PersistentVolumeClaim.ClaimName}, pvc)
			if err != nil {
				return err
			}
			if _, ok := pvc.Labels[appsv1alpha1.CloneSetInstanceID]; !ok {
				return nil
			}
			utilasi.ReplaceOwnerRef(pvc, owner)
			return gClient.Update(context.TODO(), pvc)
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *asiControl) waitForInformerWatched(oldPod *v1.Pod) {
	for i := 0; i < 3; i++ {
		time.Sleep(time.Millisecond * 5)

		got := v1.Pod{}
		if err := gClient.Get(context.TODO(), types.NamespacedName{Namespace: oldPod.Namespace, Name: oldPod.Name}, &got); err != nil {
			klog.Warningf("CloneSet %s/%s pooling failed to wait for pod %s watched: %v", c.Namespace, c.Name, oldPod.Name, err)
			return
		}

		if got.Generation > oldPod.Generation || got.ResourceVersion != oldPod.ResourceVersion {
			return
		}
	}

	klog.Warningf("CloneSet %s/%s pooling failed to wait for pod %s watched timeout", c.Namespace, c.Name, oldPod.Name)
}

func getPoolsWithReplicas(replicas int, poolConfig *appsv1alpha1.PoolConfig) map[string]int {
	poolMap := make(map[string]int, len(poolConfig.Pools))
	leftReplicas := replicas
	for _, p := range poolConfig.Pools {
		if p.Percent > 0 {
			num := integer.IntMin(round(float64(replicas)*float64(p.Percent)/100), leftReplicas)
			poolMap[p.Name] = num
			leftReplicas -= num
		}
	}

	leftPoolNum := len(poolConfig.Pools) - len(poolMap)
	if leftPoolNum > 0 {
		for i := 0; i < len(poolConfig.Pools); i++ {
			p := &poolConfig.Pools[i]
			if _, ok := poolMap[p.Name]; ok {
				continue
			}

			if i == len(poolConfig.Pools)-1 {
				poolMap[p.Name] = leftReplicas
			} else {
				num := integer.IntMin(leftReplicas/leftPoolNum, leftReplicas)
				poolMap[p.Name] = num
				leftReplicas -= num
			}
		}
	}

	return poolMap
}

func round(x float64) int {
	return int(math.Floor(x + 0.5))
}

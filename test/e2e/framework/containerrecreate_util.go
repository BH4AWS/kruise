/*
Copyright 2021 The Kruise Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package framework

import (
	"context"
	"fmt"
	"time"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"

	appsv1alpha1 "github.com/openkruise/kruise/apis/apps/v1alpha1"
	policyv1alpha1 "github.com/openkruise/kruise/apis/policy/v1alpha1"
	kruiseclientset "github.com/openkruise/kruise/pkg/client/clientset/versioned"
)

type ContainerRecreateTester struct {
	c  clientset.Interface
	kc kruiseclientset.Interface
	ns string
}

func NewContainerRecreateTester(c clientset.Interface, kc kruiseclientset.Interface, ns string) *ContainerRecreateTester {
	return &ContainerRecreateTester{
		c:  c,
		kc: kc,
		ns: ns,
	}
}

func (t *ContainerRecreateTester) CreateTestCloneSetAndGetPods(randStr string, replicas int32, containers []v1.Container) (pods []*v1.Pod) {
	set := &appsv1alpha1.CloneSet{
		ObjectMeta: metav1.ObjectMeta{Namespace: t.ns, Name: "clone-foo-" + randStr},
		Spec: appsv1alpha1.CloneSetSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"rand": randStr}},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"rand": randStr},
				},
				Spec: v1.PodSpec{
					Affinity: &v1.Affinity{
						PodAntiAffinity: &v1.PodAntiAffinity{
							PreferredDuringSchedulingIgnoredDuringExecution: []v1.WeightedPodAffinityTerm{
								{
									Weight:          100,
									PodAffinityTerm: v1.PodAffinityTerm{TopologyKey: v1.LabelHostname, LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"rand": randStr}}},
								},
							},
						},
					},
					Containers: containers,
				},
			},
		},
	}

	var err error
	if _, err = t.kc.AppsV1alpha1().CloneSets(t.ns).Create(context.TODO(), set, metav1.CreateOptions{}); err != nil {
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
	}

	// Wait for 60s
	gomega.Eventually(func() int32 {
		set, err = t.kc.AppsV1alpha1().CloneSets(t.ns).Get(context.TODO(), set.Name, metav1.GetOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		return set.Status.ReadyReplicas
	}, 120*time.Second, 3*time.Second).Should(gomega.Equal(replicas))

	podList, err := t.c.CoreV1().Pods(t.ns).List(context.TODO(), metav1.ListOptions{LabelSelector: "rand=" + randStr})
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	for i := range podList.Items {
		p := &podList.Items[i]
		pods = append(pods, p)
	}
	return
}

func (t *ContainerRecreateTester) CreateTestStatefulSetAndGetAHalfPods(stsPrefixName, randStr string, replicas int32) (pods []*v1.Pod) {
	setName := stsPrefixName + randStr
	set := &appsv1alpha1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Namespace: t.ns, Name: setName},
		Spec: appsv1alpha1.StatefulSetSpec{
			PodManagementPolicy: apps.ParallelPodManagement,
			Replicas:            &replicas,
			Selector:            &metav1.LabelSelector{MatchLabels: map[string]string{"rand": randStr}},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"rand": randStr},
				},
				Spec: v1.PodSpec{
					Affinity: &v1.Affinity{
						PodAntiAffinity: &v1.PodAntiAffinity{
							PreferredDuringSchedulingIgnoredDuringExecution: []v1.WeightedPodAffinityTerm{
								{
									Weight:          100,
									PodAffinityTerm: v1.PodAffinityTerm{TopologyKey: v1.LabelHostname, LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"rand": randStr}}},
								},
							},
						},
					},
					Containers: []v1.Container{
						{
							Command: []string{
								"/bin/sh",
								"-c",
								fmt.Sprintf("%v%v%v", `podIndex=$(echo $POD_NAME | sed 's/`, fmt.Sprintf("%v-", setName),
									`//g');podIndex=$((podIndex));if [ $((podIndex % 2)) -eq 0 ]; then sleep 2000; fi`),
							},
							Env: []v1.EnvVar{
								{
									Name: "POD_NAME",
									ValueFrom: &v1.EnvVarSource{
										FieldRef: &v1.ObjectFieldSelector{
											FieldPath: "metadata.name",
										},
									},
								},
							},
							Name:  "app",
							Image: "busybox:latest",
						},
					},
				},
			},
		},
	}

	var err error
	if _, err = t.kc.AppsV1alpha1().StatefulSets(t.ns).Create(context.TODO(), set, metav1.CreateOptions{}); err != nil {
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
	}
	ginkgo.By("Finished to create StatefulSet and wait part pods Ready")

	// Wait for 60s，in this method, just a half part of pods will be ready
	gomega.Eventually(func() int32 {
		set, err = t.kc.AppsV1alpha1().StatefulSets(t.ns).Get(context.TODO(), set.Name, metav1.GetOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		return set.Status.ReadyReplicas
	}, 120*time.Second, 3*time.Second).Should(gomega.Equal(((replicas - 1) / 2) + 1))

	podList, err := t.c.CoreV1().Pods(t.ns).List(context.TODO(), metav1.ListOptions{LabelSelector: "rand=" + randStr})
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	for i := range podList.Items {
		p := &podList.Items[i]
		pods = append(pods, p)
	}
	return
}

func (t *ContainerRecreateTester) CreateTestPubStrategy(randStr string, MaxUnavailable intstr.IntOrString) *policyv1alpha1.PodUnavailableBudget {
	pub := &policyv1alpha1.PodUnavailableBudget{
		TypeMeta: metav1.TypeMeta{
			APIVersion: policyv1alpha1.GroupVersion.String(),
			Kind:       "PodUnavailableBudget",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: t.ns,
			Name:      "pub-foo-" + randStr,
		},
		Spec: policyv1alpha1.PodUnavailableBudgetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"rand": randStr,
				},
			},
			MaxUnavailable: &MaxUnavailable,
		},
	}
	Logf("create PodUnavailableBudget(%s/%s)", pub.Namespace, pub.Name)
	_, err := t.kc.PolicyV1alpha1().PodUnavailableBudgets(pub.Namespace).Create(context.TODO(), pub, metav1.CreateOptions{})
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	Logf("waiting for pub strategy to enter running")
	pollErr := wait.PollImmediate(time.Second, time.Minute,
		func() (bool, error) {
			_, err := t.kc.PolicyV1alpha1().PodUnavailableBudgets(pub.Namespace).Get(context.TODO(), pub.Name, metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			return true, nil
		})
	if pollErr != nil {
		Failf("Failed waiting for PodUnavailableBudget to enter running: %v", pollErr)
	}
	pub, _ = t.kc.PolicyV1alpha1().PodUnavailableBudgets(pub.Namespace).Get(context.TODO(), pub.Name, metav1.GetOptions{})
	return pub

}

func (t *ContainerRecreateTester) CleanAllTestResources() error {
	if err := t.kc.PolicyV1alpha1().PodUnavailableBudgets(t.ns).DeleteCollection(context.TODO(), metav1.DeleteOptions{}, metav1.ListOptions{}); err != nil {
		return err
	}
	if err := t.kc.AppsV1alpha1().ContainerRecreateRequests(t.ns).DeleteCollection(context.TODO(), metav1.DeleteOptions{}, metav1.ListOptions{}); err != nil {
		return err
	}
	if err := t.kc.AppsV1alpha1().CloneSets(t.ns).DeleteCollection(context.TODO(), metav1.DeleteOptions{}, metav1.ListOptions{}); err != nil {
		return err
	}
	return nil
}

func (t *ContainerRecreateTester) CreateCRR(crr *appsv1alpha1.ContainerRecreateRequest) (*appsv1alpha1.ContainerRecreateRequest, error) {
	return t.kc.AppsV1alpha1().ContainerRecreateRequests(crr.Namespace).Create(context.TODO(), crr, metav1.CreateOptions{})
}

func (t *ContainerRecreateTester) GetCRR(name string) (*appsv1alpha1.ContainerRecreateRequest, error) {
	return t.kc.AppsV1alpha1().ContainerRecreateRequests(t.ns).Get(context.TODO(), name, metav1.GetOptions{})
}

func (t *ContainerRecreateTester) GetPod(name string) (*v1.Pod, error) {
	return t.c.CoreV1().Pods(t.ns).Get(context.TODO(), name, metav1.GetOptions{})
}

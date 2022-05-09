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

package policy

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	appsv1alpha1 "github.com/openkruise/kruise/apis/apps/v1alpha1"
	kruiseclientset "github.com/openkruise/kruise/pkg/client/clientset/versioned"
	"github.com/openkruise/kruise/pkg/control/pubcontrol"
	"github.com/openkruise/kruise/test/e2e/framework"
	sigmakruiseapi "gitlab.alibaba-inc.com/sigma/sigma-k8s-api/pkg/kruise"
	corev1 "k8s.io/api/core/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	utilpointer "k8s.io/utils/pointer"
)

const (
	// namespace labels
	NamespaceEnablePubLabel = "kruise.io/enable-pub-strategy"
)

var _ = SIGDescribe("PodUnavailableBudgetASI", func() {
	f := framework.NewDefaultFramework("podunavailablebudget")
	var ns string
	var c clientset.Interface
	var kc kruiseclientset.Interface
	var tester *framework.PodUnavailableBudgetTester
	var sidecarTester *framework.SidecarSetTester

	ginkgo.BeforeEach(func() {
		c = f.ClientSet
		kc = f.KruiseClientSet
		ns = f.Namespace.Name
		tester = framework.NewPodUnavailableBudgetTester(c, kc)
		sidecarTester = framework.NewSidecarSetTester(c, kc)
	})

	framework.KruiseDescribe("podUnavailableBudget functionality [podUnavailableBudget]", func() {

		ginkgo.AfterEach(func() {
			if ginkgo.CurrentGinkgoTestDescription().Failed {
				framework.DumpDebugInfo(c, ns)
			}
			framework.Logf("Deleting all PodUnavailableBudgets and Deployments in cluster")
			tester.DeletePubs(ns)
			tester.DeleteDeployments(ns)
			tester.DeleteCloneSets(ns)
			sidecarTester.DeleteSidecarSets()
		})

		ginkgo.It("PodUnavailableBudget asi cloneSet", func() {
			ginkgo.By("PodUnavailableBudget asi cloneSet start...")
			// create deployment
			cloneset := tester.NewBaseCloneSet(ns)
			cloneset.Spec.Replicas = utilpointer.Int32Ptr(2)
			cloneset.Labels = map[string]string{
				sigmakruiseapi.LabelCloneSetMode:  sigmakruiseapi.CloneSetASI,
				"sigma.ali/app-name":              "ele-base-solon",
				"sigma.ali/instance-group":        "ele-base-solon-asi-deploy-test",
				"sigma.ali/site":                  "nt12",
				"sigma.alibaba-inc.com/app-stage": "DAILY",
				"sigma.alibaba-inc.com/app-unit":  "CENTER_UNIT.center",
			}
			cloneset.Spec.UpdateStrategy = appsv1alpha1.CloneSetUpdateStrategy{
				Type: appsv1alpha1.InPlaceOnlyCloneSetUpdateStrategyType,
			}
			cloneset.Spec.Template.Spec.Containers[0].Image = "reg.docker.alibaba-inc.com/base/busybox:latest"
			cloneset.Spec.Template.Spec.Tolerations = newAsiTolerations()
			cloneset.Spec.Template.Labels = map[string]string{
				"sigma.ali/app-name":              "ele-base-solon",
				"sigma.ali/instance-group":        "ele-base-solon-asi-deploy-test",
				"sigma.ali/site":                  "nt12",
				"sigma.alibaba-inc.com/app-stage": "DAILY",
				"sigma.alibaba-inc.com/app-unit":  "CENTER_UNIT.center",
				"app":                             "busybox",
				"pub-controller":                  "true",
			}
			by, _ := json.Marshal(cloneset)
			fmt.Println(string(by))
			ginkgo.By(fmt.Sprintf("Creating CloneSet(%s.%s)", cloneset.Namespace, cloneset.Name))
			cloneset = tester.CreateCloneSet(cloneset)
			nsObj, err := c.CoreV1().Namespaces().Get(context.TODO(), ns, metav1.GetOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			nsObj.Labels = map[string]string{
				NamespaceEnablePubLabel: "true",
			}
			_, err = c.CoreV1().Namespaces().Update(context.TODO(), nsObj, metav1.UpdateOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			time.Sleep(time.Second * 3)
			pubName := fmt.Sprintf("%s-pub", cloneset.Name)
			pub, err := kc.PolicyV1alpha1().PodUnavailableBudgets(ns).Get(context.TODO(), pubName, metav1.GetOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(pub.Spec.MaxUnavailable.IntVal).To(gomega.Equal(int32(1)))
			gomega.Expect(pub.Status.TotalReplicas).To(gomega.Equal(int32(2)))
			gomega.Expect(pub.Status.CurrentAvailable).To(gomega.Equal(int32(2)))
			gomega.Expect(pub.Status.UnavailableAllowed).To(gomega.Equal(int32(1)))
			gomega.Expect(pub.Status.DesiredAvailable).To(gomega.Equal(int32(1)))
			pods, err := sidecarTester.GetSelectorPods(cloneset.Namespace, cloneset.Spec.Selector)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(pods).To(gomega.HaveLen(2))
			for i := range pods {
				podIn := pods[i]
				// pub会防护 pub 删除
				err = c.CoreV1().Pods(ns).Delete(context.TODO(), podIn.Name, metav1.DeleteOptions{})
				if i == 0 {
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				} else {
					gomega.Expect(err).Should(gomega.HaveOccurred())
				}
			}
			pub, err = kc.PolicyV1alpha1().PodUnavailableBudgets(ns).Get(context.TODO(), pubName, metav1.GetOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(pub.Status.UnavailableAllowed).To(gomega.Equal(int32(0)))

			// scale up cloneSet.replicas=4
			cloneset, err = kc.AppsV1alpha1().CloneSets(ns).Get(context.TODO(), cloneset.Name, metav1.GetOptions{})
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			cloneset.Spec.Replicas = utilpointer.Int32Ptr(4)
			_, err = kc.AppsV1alpha1().CloneSets(cloneset.Namespace).Update(context.TODO(), cloneset, metav1.UpdateOptions{})
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			time.Sleep(time.Second * 10)
			pub, err = kc.PolicyV1alpha1().PodUnavailableBudgets(ns).Get(context.TODO(), pubName, metav1.GetOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(pub.Spec.MaxUnavailable.IntVal).To(gomega.Equal(int32(2)))
			gomega.Expect(pub.Status.TotalReplicas).To(gomega.Equal(int32(4)))
			gomega.Expect(pub.Status.CurrentAvailable).To(gomega.Equal(int32(4)))
			gomega.Expect(pub.Status.UnavailableAllowed).To(gomega.Equal(int32(2)))
			gomega.Expect(pub.Status.DesiredAvailable).To(gomega.Equal(int32(2)))
			pods, err = sidecarTester.GetSelectorPods(cloneset.Namespace, cloneset.Spec.Selector)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(pods).To(gomega.HaveLen(4))
			for i := range pods {
				podIn := pods[i]
				// pub会防护 pub 删除
				err = c.CoreV1().Pods(ns).Delete(context.TODO(), podIn.Name, metav1.DeleteOptions{})
				if i < 2 {
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				} else {
					gomega.Expect(err).Should(gomega.HaveOccurred())
				}
			}
			pub, err = kc.PolicyV1alpha1().PodUnavailableBudgets(ns).Get(context.TODO(), pubName, metav1.GetOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(pub.Status.UnavailableAllowed).To(gomega.Equal(int32(0)))
			time.Sleep(time.Second * 5)

			// 快上快下场景
			pods, err = sidecarTester.GetSelectorPods(cloneset.Namespace, cloneset.Spec.Selector)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(pods).To(gomega.HaveLen(4))
			// set two pods wait_online
			for i := range pods {
				podIn := pods[i]
				if i >= 2 {
					break
				}
				podIn.Labels[pubcontrol.PodNamingRegisterStateLabel] = pubcontrol.PodWaitOnlineValue
				_, err = c.CoreV1().Pods(ns).Update(context.TODO(), podIn, metav1.UpdateOptions{})
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			}
			time.Sleep(time.Second * 10)
			pub, err = kc.PolicyV1alpha1().PodUnavailableBudgets(ns).Get(context.TODO(), pubName, metav1.GetOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(pub.Spec.MaxUnavailable.IntVal).To(gomega.Equal(int32(1)))
			gomega.Expect(pub.Status.TotalReplicas).To(gomega.Equal(int32(2)))
			gomega.Expect(pub.Status.CurrentAvailable).To(gomega.Equal(int32(2)))
			gomega.Expect(pub.Status.UnavailableAllowed).To(gomega.Equal(int32(1)))
			gomega.Expect(pub.Status.DesiredAvailable).To(gomega.Equal(int32(1)))

			pods, err = sidecarTester.GetSelectorPods(cloneset.Namespace, cloneset.Spec.Selector)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(pods).To(gomega.HaveLen(4))
			deleteOnlineNumber := 0
			for _, podIn := range pods {
				if podIn.Labels[pubcontrol.PodNamingRegisterStateLabel] == pubcontrol.PodWaitOnlineValue {
					continue
				}
				deleteOnlineNumber++
				// pub只防护 online pod
				err = c.CoreV1().Pods(ns).Delete(context.TODO(), podIn.Name, metav1.DeleteOptions{})
				if deleteOnlineNumber == 2 {
					gomega.Expect(err).Should(gomega.HaveOccurred())
				} else {
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				}
			}
			time.Sleep(time.Second * 5)
			// 删除 wait_online pod 不防护
			pods, err = sidecarTester.GetSelectorPods(cloneset.Namespace, cloneset.Spec.Selector)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(pods).To(gomega.HaveLen(4))
			for _, podIn := range pods {
				if podIn.Labels[pubcontrol.PodNamingRegisterStateLabel] != pubcontrol.PodWaitOnlineValue {
					continue
				}
				// pub只防护 online pod
				err = c.CoreV1().Pods(ns).Delete(context.TODO(), podIn.Name, metav1.DeleteOptions{})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			}
			time.Sleep(time.Second * 10)
			pub, err = kc.PolicyV1alpha1().PodUnavailableBudgets(ns).Get(context.TODO(), pubName, metav1.GetOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(pub.Spec.MaxUnavailable.IntVal).To(gomega.Equal(int32(2)))
			gomega.Expect(pub.Status.TotalReplicas).To(gomega.Equal(int32(4)))
			gomega.Expect(pub.Status.CurrentAvailable).To(gomega.Equal(int32(4)))
			gomega.Expect(pub.Status.UnavailableAllowed).To(gomega.Equal(int32(2)))
			gomega.Expect(pub.Status.DesiredAvailable).To(gomega.Equal(int32(2)))

			// 验证evict 防护
			pods, err = sidecarTester.GetSelectorPods(cloneset.Namespace, cloneset.Spec.Selector)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(pods).To(gomega.HaveLen(4))
			for i := range pods {
				podIn := pods[i]
				// pub会防护 pub eviction
				err = c.CoreV1().Pods(ns).Evict(context.TODO(), &policyv1beta1.Eviction{ObjectMeta: metav1.ObjectMeta{Name: podIn.Name}})
				if i < 2 {
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				} else {
					fmt.Println(err.Error())
					gomega.Expect(err).Should(gomega.HaveOccurred())
				}
			}

			ginkgo.By("PodUnavailableBudget asi cloneSet success")
		})
	})
})

func newAsiTolerations() []corev1.Toleration {
	tolerations := []corev1.Toleration{
		{
			Key:      "sigma.ali/is-ecs",
			Operator: corev1.TolerationOpExists,
		},
		{
			Key:      "sigma.ali/resource-pool",
			Operator: corev1.TolerationOpExists,
			Effect:   corev1.TaintEffectNoSchedule,
		},
		{
			Key:               "node.kubernetes.io/not-ready",
			Operator:          corev1.TolerationOpExists,
			Effect:            corev1.TaintEffectNoExecute,
			TolerationSeconds: utilpointer.Int64Ptr(300),
		},
		{
			Key:               "node.kubernetes.io/unreachable",
			Operator:          corev1.TolerationOpExists,
			Effect:            corev1.TaintEffectNoExecute,
			TolerationSeconds: utilpointer.Int64Ptr(300),
		},
	}

	return tolerations
}

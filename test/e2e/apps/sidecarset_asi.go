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

package apps

import (
	"fmt"
	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	appsv1alpha1 "github.com/openkruise/kruise/apis/apps/v1alpha1"
	kruiseclientset "github.com/openkruise/kruise/pkg/client/clientset/versioned"
	"github.com/openkruise/kruise/pkg/control/sidecarcontrol"
	"github.com/openkruise/kruise/pkg/util"
	"github.com/openkruise/kruise/pkg/utilasi/commontypes"
	"github.com/openkruise/kruise/test/e2e/framework"
	sigmak8sapi "gitlab.alibaba-inc.com/sigma/sigma-k8s-api/pkg/api"
	sigmakruiseapi "gitlab.alibaba-inc.com/sigma/sigma-k8s-api/pkg/kruise"
	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	clientset "k8s.io/client-go/kubernetes"
	utilpointer "k8s.io/utils/pointer"
	"time"
)

var _ = SIGDescribe("sidecarset-asi", func() {
	f := framework.NewDefaultFramework("sidecarset-asi")
	var ns string
	var c clientset.Interface
	var kc kruiseclientset.Interface
	var tester *framework.SidecarSetTester

	ginkgo.BeforeEach(func() {
		c = f.ClientSet
		kc = f.KruiseClientSet
		ns = f.Namespace.Name
		tester = framework.NewSidecarSetTester(c, kc)
	})

	framework.KruiseDescribe("SidecarSet Injecting functionality [SidecarSetInject]", func() {

		ginkgo.AfterEach(func() {
			if ginkgo.CurrentGinkgoTestDescription().Failed {
				framework.DumpDebugInfo(c, ns)
			}
			framework.Logf("Deleting all SidecarSet in cluster")
			tester.DeleteSidecarSets()
			tester.DeleteDeployments(ns)
		})

		ginkgo.It("pods don't have matched sidecarSet", func() {
			// create sidecarSet
			sidecarSet := tester.NewBaseSidecarSet(ns)
			sidecarSet.Labels = map[string]string{sidecarcontrol.LabelSidecarSetMode: sidecarcontrol.SidecarSetASI}
			// sidecarSet no matched pods
			sidecarSet.Spec.Selector.MatchLabels["app"] = "nomatched"
			ginkgo.By(fmt.Sprintf("Creating SidecarSet %s", sidecarSet.Name))
			tester.CreateSidecarSet(sidecarSet)

			// create deployment
			deployment := tester.NewBaseDeployment(ns)
			deployment.Spec.Template.Spec.Containers[0].Image = "reg.docker.alibaba-inc.com/base/busybox:latest"
			deployment.Spec.Template.Spec.Tolerations = newAsiTolerations()
			ginkgo.By(fmt.Sprintf("Creating Deployment(%s.%s)", deployment.Namespace, deployment.Name))
			tester.CreateDeployment(deployment)

			// get pods
			pods, err := tester.GetSelectorPods(deployment.Namespace, deployment.Spec.Selector)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(pods).To(gomega.HaveLen(int(*deployment.Spec.Replicas)))
			pod := pods[0]
			gomega.Expect(pod.Spec.Containers).To(gomega.HaveLen(len(deployment.Spec.Template.Spec.Containers)))
			ginkgo.By(fmt.Sprintf("test no matched sidecarSet done"))
		})

		ginkgo.It("sidecarSet inject pod sidecar container", func() {
			// create sidecarSet
			sidecarSet := tester.NewBaseSidecarSet(ns)
			sidecarSet.Labels = map[string]string{sidecarcontrol.LabelSidecarSetMode: sidecarcontrol.SidecarSetASI}
			sidecarSet.Spec.InitContainers[0].Image = "reg.docker.alibaba-inc.com/busybox:latest"
			sidecarSet.Spec.Containers[0].Image = "reg.docker.alibaba-inc.com/nginx:latest"
			sidecarSet.Spec.Containers[1].Image = "reg.docker.alibaba-inc.com/busybox:latest"
			ginkgo.By(fmt.Sprintf("Creating SidecarSet %s", sidecarSet.Name))
			tester.CreateSidecarSet(sidecarSet)

			// create deployment
			deployment := tester.NewBaseDeployment(ns)
			deployment.Spec.Template.Spec.Containers[0].Image = "reg.docker.alibaba-inc.com/base/busybox:latest"
			deployment.Spec.Template.Spec.Tolerations = newAsiTolerations()
			ginkgo.By(fmt.Sprintf("Creating Deployment(%s.%s)", deployment.Namespace, deployment.Name))
			tester.CreateDeployment(deployment)

			// get pods
			pods, err := tester.GetSelectorPods(deployment.Namespace, deployment.Spec.Selector)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			pod := pods[0]
			gomega.Expect(pod.Spec.Containers).To(gomega.HaveLen(len(deployment.Spec.Template.Spec.Containers) + len(sidecarSet.Spec.Containers)))
			gomega.Expect(pod.Spec.InitContainers).To(gomega.HaveLen(len(deployment.Spec.Template.Spec.InitContainers) + len(sidecarSet.Spec.InitContainers)))
			exceptContainers := []string{"nginx-sidecar", "main", "busybox-sidecar"}
			for i, except := range exceptContainers {
				gomega.Expect(except).To(gomega.Equal(pod.Spec.Containers[i].Name))
			}
			ginkgo.By(fmt.Sprintf("sidecarSet inject pod sidecar container done"))
		})

		ginkgo.It("sidecarSet inject pod sidecar container volumeMounts", func() {
			// create sidecarSet
			sidecarSet := tester.NewBaseSidecarSet(ns)
			sidecarSet.Labels = map[string]string{sidecarcontrol.LabelSidecarSetMode: sidecarcontrol.SidecarSetASI}
			sidecarSet.Spec.InitContainers[0].Image = "reg.docker.alibaba-inc.com/busybox:latest"
			sidecarSet.Spec.Containers[0].Image = "reg.docker.alibaba-inc.com/nginx:latest"
			sidecarSet.Spec.Containers[1].Image = "reg.docker.alibaba-inc.com/busybox:latest"
			// create deployment
			deployment := tester.NewBaseDeployment(ns)
			deployment.Spec.Template.Spec.Containers[0].Image = "reg.docker.alibaba-inc.com/base/busybox:latest"
			deployment.Spec.Template.Spec.Tolerations = newAsiTolerations()

			cases := []struct {
				name               string
				getDeployment      func() *apps.Deployment
				getSidecarSets     func() *appsv1alpha1.SidecarSet
				exceptVolumeMounts []string
				exceptEnvs         []string
				exceptVolumes      []string
			}{
				{
					name: "append normal volumeMounts, ShareVolumePolicy.Type=false",
					getDeployment: func() *apps.Deployment {
						deployIn := deployment.DeepCopy()
						deployIn.Spec.Template.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{
							{
								Name:      "main-volume",
								MountPath: "/main-volume",
							},
							{
								Name:      "log",
								MountPath: commontypes.MntVarlog,
							},
						}
						deployIn.Spec.Template.Spec.Volumes = []corev1.Volume{
							{
								Name: "main-volume",
								VolumeSource: corev1.VolumeSource{
									EmptyDir: &corev1.EmptyDirVolumeSource{},
								},
							},
							{
								Name: "log",
								VolumeSource: corev1.VolumeSource{
									EmptyDir: &corev1.EmptyDirVolumeSource{},
								},
							},
						}
						return deployIn
					},
					getSidecarSets: func() *appsv1alpha1.SidecarSet {
						sidecarSetIn := sidecarSet.DeepCopy()
						sidecarSetIn.Spec.Containers = sidecarSetIn.Spec.Containers[:1]
						sidecarSetIn.Spec.Containers[0].ShareVolumePolicy.Type = appsv1alpha1.ShareVolumePolicyDisabled
						sidecarSetIn.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{
							{
								Name:      "nginx-volume",
								MountPath: "/nginx-volume",
							},
						}
						sidecarSetIn.Spec.Volumes = []corev1.Volume{
							{
								Name: "nginx-volume",
								VolumeSource: corev1.VolumeSource{
									EmptyDir: &corev1.EmptyDirVolumeSource{},
								},
							},
						}
						return sidecarSetIn
					},
					exceptVolumeMounts: []string{"/nginx-volume"},
					exceptVolumes:      []string{"main-volume", "nginx-volume"},
				},
				{
					name: "append normal volumeMounts, ShareVolumePolicy.Type=true",
					getDeployment: func() *apps.Deployment {
						deployIn := deployment.DeepCopy()
						deployIn.Spec.Template.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{
							{
								Name:      "main-volume",
								MountPath: "/main-volume",
							},
							{
								Name:      "log",
								MountPath: commontypes.MntVarlog,
							},
						}
						deployIn.Spec.Template.Spec.Volumes = []corev1.Volume{
							{
								Name: "main-volume",
								VolumeSource: corev1.VolumeSource{
									EmptyDir: &corev1.EmptyDirVolumeSource{},
								},
							},
							{
								Name: "log",
								VolumeSource: corev1.VolumeSource{
									EmptyDir: &corev1.EmptyDirVolumeSource{},
								},
							},
						}
						return deployIn
					},
					getSidecarSets: func() *appsv1alpha1.SidecarSet {
						sidecarSetIn := sidecarSet.DeepCopy()
						sidecarSetIn.Spec.Containers = sidecarSetIn.Spec.Containers[:1]
						sidecarSetIn.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{
							{
								Name:      "nginx-volume",
								MountPath: "/nginx-volume",
							},
						}
						sidecarSetIn.Spec.Volumes = []corev1.Volume{
							{
								Name: "nginx-volume",
								VolumeSource: corev1.VolumeSource{
									EmptyDir: &corev1.EmptyDirVolumeSource{},
								},
							},
						}
						return sidecarSetIn
					},
					exceptVolumeMounts: []string{"/main-volume", "/nginx-volume", commontypes.MntServiceAccount},
					exceptVolumes:      []string{"main-volume", "nginx-volume"},
				},
			}

			for _, cs := range cases {
				ginkgo.By(cs.name)
				sidecarSetIn := cs.getSidecarSets()
				ginkgo.By(fmt.Sprintf("Creating SidecarSet %s", sidecarSetIn.Name))
				tester.CreateSidecarSet(sidecarSetIn)
				deploymentIn := cs.getDeployment()
				ginkgo.By(fmt.Sprintf("Creating Deployment(%s.%s)", deploymentIn.Namespace, deploymentIn.Name))
				tester.CreateDeployment(deploymentIn)
				// get pods
				pods, err := tester.GetSelectorPods(deploymentIn.Namespace, deploymentIn.Spec.Selector)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				// volume
				for _, volume := range cs.exceptVolumes {
					object := util.GetPodVolume(&pods[0], volume)
					gomega.Expect(object).ShouldNot(gomega.BeNil())
				}
				// volumeMounts
				sidecarContainer := &pods[0].Spec.Containers[0]
				for _, volumeMount := range cs.exceptVolumeMounts {
					object := util.GetContainerVolumeMount(sidecarContainer, volumeMount)
					gomega.Expect(object).ShouldNot(gomega.BeNil())
				}
				gomega.Expect(sidecarContainer.VolumeMounts).To(gomega.HaveLen(len(cs.exceptVolumeMounts)))
				// envs
				for _, env := range cs.exceptEnvs {
					object := util.GetContainerEnvVar(sidecarContainer, env)
					gomega.Expect(object).ShouldNot(gomega.BeNil())
				}
			}
			ginkgo.By(fmt.Sprintf("sidecarSet inject pod sidecar container volumeMounts done"))
		})

		ginkgo.It("sidecarSet inject pod sidecar container volumeMounts, SubPathExpr with expanded subpath", func() {
			// create sidecarSet
			sidecarSet := tester.NewBaseSidecarSet(ns)
			sidecarSet.Labels = map[string]string{sidecarcontrol.LabelSidecarSetMode: sidecarcontrol.SidecarSetASI}
			sidecarSet.Spec.InitContainers[0].Image = "reg.docker.alibaba-inc.com/busybox:latest"
			sidecarSet.Spec.Containers[0].Image = "reg.docker.alibaba-inc.com/nginx:latest"
			sidecarSet.Spec.Containers[1].Image = "reg.docker.alibaba-inc.com/busybox:latest"
			// create deployment
			deployment := tester.NewBaseDeployment(ns)
			deployment.Spec.Template.Spec.Containers[0].Image = "reg.docker.alibaba-inc.com/base/busybox:latest"
			deployment.Spec.Template.Spec.Tolerations = newAsiTolerations()

			cases := []struct {
				name               string
				getDeployment      func() *apps.Deployment
				getSidecarSets     func() *appsv1alpha1.SidecarSet
				exceptVolumeMounts []string
				exceptEnvs         []string
				exceptVolumes      []string
			}{
				{
					name: "append volumeMounts SubPathExpr, volumes with expanded subpath",
					getDeployment: func() *apps.Deployment {
						deployIn := deployment.DeepCopy()
						deployIn.Spec.Template.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{
							{
								Name:        "main-volume",
								MountPath:   "/main-volume",
								SubPathExpr: "foo/$(POD_NAME)/$(OD_NAME)/conf",
							},
						}
						deployIn.Spec.Template.Spec.Containers[0].Env = []corev1.EnvVar{
							{
								Name:  "POD_NAME",
								Value: "bar",
							},
							{
								Name:  "OD_NAME",
								Value: "od_name",
							},
						}
						deployIn.Spec.Template.Spec.Volumes = []corev1.Volume{
							{
								Name: "main-volume",
								VolumeSource: corev1.VolumeSource{
									EmptyDir: &corev1.EmptyDirVolumeSource{},
								},
							},
						}
						return deployIn
					},
					getSidecarSets: func() *appsv1alpha1.SidecarSet {
						sidecarSetIn := sidecarSet.DeepCopy()
						sidecarSetIn.Spec.Containers = sidecarSetIn.Spec.Containers[:1]
						sidecarSetIn.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{
							{
								Name:      "nginx-volume",
								MountPath: "/nginx-volume",
							},
						}
						sidecarSetIn.Spec.Volumes = []corev1.Volume{
							{
								Name: "nginx-volume",
								VolumeSource: corev1.VolumeSource{
									EmptyDir: &corev1.EmptyDirVolumeSource{},
								},
							},
						}
						return sidecarSetIn
					},
					exceptVolumeMounts: []string{"/main-volume", "/nginx-volume"},
					exceptVolumes:      []string{"main-volume", "nginx-volume"},
					exceptEnvs:         []string{"POD_NAME", "OD_NAME"},
				},
			}

			for _, cs := range cases {
				ginkgo.By(cs.name)
				sidecarSetIn := cs.getSidecarSets()
				ginkgo.By(fmt.Sprintf("Creating SidecarSet %s", sidecarSetIn.Name))
				tester.CreateSidecarSet(sidecarSetIn)
				deploymentIn := cs.getDeployment()
				ginkgo.By(fmt.Sprintf("Creating Deployment(%s.%s)", deploymentIn.Namespace, deploymentIn.Name))
				tester.CreateDeployment(deploymentIn)
				// get pods
				pods, err := tester.GetSelectorPods(deploymentIn.Namespace, deploymentIn.Spec.Selector)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				// volume
				for _, volume := range cs.exceptVolumes {
					object := util.GetPodVolume(&pods[0], volume)
					gomega.Expect(object).ShouldNot(gomega.BeNil())
				}
				// volumeMounts
				sidecarContainer := &pods[0].Spec.Containers[0]
				for _, volumeMount := range cs.exceptVolumeMounts {
					object := util.GetContainerVolumeMount(sidecarContainer, volumeMount)
					gomega.Expect(object).ShouldNot(gomega.BeNil())
				}
				// envs
				for _, env := range cs.exceptEnvs {
					object := util.GetContainerEnvVar(sidecarContainer, env)
					gomega.Expect(object).ShouldNot(gomega.BeNil())
				}
			}
			ginkgo.By(fmt.Sprintf("sidecarSet inject pod sidecar container volumeMounts, SubPathExpr with expanded subpath done"))
		})

		ginkgo.It("sidecarSet inject pod sidecar container transfer Envs", func() {
			// create sidecarSet
			sidecarSetIn := tester.NewBaseSidecarSet(ns)
			sidecarSetIn.Labels = map[string]string{sidecarcontrol.LabelSidecarSetMode: sidecarcontrol.SidecarSetASI}
			sidecarSetIn.Spec.InitContainers[0].Image = "reg.docker.alibaba-inc.com/busybox:latest"
			sidecarSetIn.Spec.Containers[0].Image = "reg.docker.alibaba-inc.com/nginx:latest"
			sidecarSetIn.Spec.Containers = sidecarSetIn.Spec.Containers[:1]
			sidecarSetIn.Spec.Containers[0].Env = []corev1.EnvVar{
				{
					Name:  "OD_NAME",
					Value: "sidecar_name",
				},
				{
					Name:  "SidecarName",
					Value: "nginx-sidecar",
				},
			}
			sidecarSetIn.Spec.Containers[0].TransferEnv = []appsv1alpha1.TransferEnvVar{
				{
					SourceContainerName: "main",
					EnvName:             "POD_NAME",
				},
				{
					SourceContainerName: "main",
					EnvName:             "OD_NAME",
				},
				{
					SourceContainerName: "main",
					EnvName:             "PROXY_IP",
				},
			}
			ginkgo.By(fmt.Sprintf("Creating SidecarSet %s", sidecarSetIn.Name))
			tester.CreateSidecarSet(sidecarSetIn)

			// create deployment
			deploymentIn := tester.NewBaseDeployment(ns)
			deploymentIn.Spec.Template.Spec.Containers[0].Image = "reg.docker.alibaba-inc.com/base/busybox:latest"
			deploymentIn.Spec.Template.Spec.Tolerations = newAsiTolerations()
			deploymentIn.Spec.Template.Spec.Containers[0].Env = []corev1.EnvVar{
				{
					Name:  "POD_NAME",
					Value: "bar",
				},
				{
					Name:  "OD_NAME",
					Value: "od_name",
				},
				{
					Name:  "PROXY_IP",
					Value: "127.0.0.1",
				},
			}
			ginkgo.By(fmt.Sprintf("Creating Deployment(%s.%s)", deploymentIn.Namespace, deploymentIn.Name))
			tester.CreateDeployment(deploymentIn)
			// get pods
			pods, err := tester.GetSelectorPods(deploymentIn.Namespace, deploymentIn.Spec.Selector)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			// except envs
			exceptEnvs := map[string]string{
				"POD_NAME":    "bar",
				"OD_NAME":     "sidecar_name",
				"PROXY_IP":    "127.0.0.1",
				"SidecarName": "nginx-sidecar",
			}
			sidecarContainer := &pods[0].Spec.Containers[0]
			// envs
			for key, value := range exceptEnvs {
				object := util.GetContainerEnvValue(sidecarContainer, key)
				gomega.Expect(object).To(gomega.Equal(value))
			}
			ginkgo.By(fmt.Sprintf("sidecarSet inject pod sidecar container transfer Envs done"))
		})

		ginkgo.It("sidecarSet inject pod sidecar container, on in-place update", func() {
			// create deployment
			cloneset := tester.NewBaseCloneSet(ns)
			cloneset.Spec.Replicas = utilpointer.Int32Ptr(1)
			cloneset.Labels = map[string]string{sigmakruiseapi.LabelCloneSetMode: sigmakruiseapi.CloneSetASI}
			cloneset.Spec.UpdateStrategy = appsv1alpha1.CloneSetUpdateStrategy{
				Type: appsv1alpha1.InPlaceOnlyCloneSetUpdateStrategyType,
			}
			cloneset.Spec.Template.Spec.Containers[0].Image = "reg.docker.alibaba-inc.com/base/busybox:latest"
			cloneset.Spec.Template.Spec.Tolerations = newAsiTolerations()
			ginkgo.By(fmt.Sprintf("Creating CloneSet(%s.%s)", cloneset.Namespace, cloneset.Name))
			cloneset = tester.CreateCloneSet(cloneset)
			fmt.Println("cloneSet", cloneset.ResourceVersion)

			// create sidecarSet
			sidecarSet := tester.NewBaseSidecarSet(ns)
			sidecarSet.Labels = map[string]string{sidecarcontrol.LabelSidecarSetMode: sidecarcontrol.SidecarSetASI}
			sidecarSet.Spec.InitContainers[0].Image = "reg.docker.alibaba-inc.com/busybox:latest"
			sidecarSet.Spec.Containers[0].Image = "reg.docker.alibaba-inc.com/nginx:latest"
			sidecarSet.Spec.Containers[1].Image = "reg.docker.alibaba-inc.com/busybox:latest"
			ginkgo.By(fmt.Sprintf("Creating SidecarSet %s", sidecarSet.Name))
			tester.CreateSidecarSet(sidecarSet)

			// get pods and not sidecar container
			pods, err := tester.GetSelectorPods(cloneset.Namespace, cloneset.Spec.Selector)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			pod := pods[0]
			gomega.Expect(pod.Spec.Containers).To(gomega.HaveLen(len(cloneset.Spec.Template.Spec.Containers)))
			gomega.Expect(pod.Spec.InitContainers).To(gomega.HaveLen(0))

			// update cloneset spec container volume and don't inject sidecar container
			ginkgo.By(fmt.Sprintf("in-place update cloneSet(%s.%s) for volumeMounts", cloneset.Namespace, cloneset.Name))
			cloneset.Spec.Template.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{
				{
					Name:      "main-volume",
					MountPath: "/main-volume",
				},
			}
			cloneset.Spec.Template.Spec.Volumes = []corev1.Volume{
				{
					Name: "main-volume",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			}
			_, err = kc.AppsV1alpha1().CloneSets(cloneset.Namespace).Update(cloneset)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			time.Sleep(time.Second * 10)

			// check pods and not inject sidecar container
			ginkgo.By(fmt.Sprintf("check whether cloneSet(%s.%s)'s pods no injected into sidecar containers", cloneset.Namespace, cloneset.Name))
			pods, err = tester.GetSelectorPods(cloneset.Namespace, cloneset.Spec.Selector)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			pod = pods[0]
			gomega.Expect(pod.Spec.Containers).To(gomega.HaveLen(len(cloneset.Spec.Template.Spec.Containers)))
			gomega.Expect(pod.Spec.InitContainers).To(gomega.HaveLen(0))

			// update sidecarSet annotations
			// get latest sidecarSet
			ginkgo.By(fmt.Sprintf("update sidecarSet(%s) add annotations", sidecarSet.Name))
			sidecarSet, _ = kc.AppsV1alpha1().SidecarSets().Get(sidecarSet.Name, metav1.GetOptions{})
			sidecarSet.Annotations = map[string]string{
				sidecarcontrol.SidecarInjectOnInplaceUpdate: "true",
			}
			tester.UpdateSidecarSet(sidecarSet)
			time.Sleep(time.Second)

			// update cloneset spec container envs and inject sidecar container
			ginkgo.By(fmt.Sprintf("in-place update cloneSet(%s.%s) for envs", cloneset.Namespace, cloneset.Name))
			cloneset, _ = kc.AppsV1alpha1().CloneSets(cloneset.Namespace).Get(cloneset.Name, metav1.GetOptions{})
			cloneset.Spec.Template.Spec.Containers[0].Env = []corev1.EnvVar{
				{
					Name:  "TEST_ENV",
					Value: "value-1",
				},
			}
			_, err = kc.AppsV1alpha1().CloneSets(cloneset.Namespace).Update(cloneset)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			time.Sleep(time.Second * 10)

			// check pods and inject sidecar container
			ginkgo.By(fmt.Sprintf("check whether cloneSet(%s.%s)'s pods injected into sidecar containers", cloneset.Namespace, cloneset.Name))
			pods, err = tester.GetSelectorPods(cloneset.Namespace, cloneset.Spec.Selector)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			pod = pods[0]
			gomega.Expect(pod.Spec.Containers).To(gomega.HaveLen(len(cloneset.Spec.Template.Spec.Containers) + len(sidecarSet.Spec.Containers)))
			gomega.Expect(pod.Spec.InitContainers).To(gomega.HaveLen(0))

			exceptContainers := []string{"nginx-sidecar", "main", "busybox-sidecar"}
			for i, except := range exceptContainers {
				gomega.Expect(except).To(gomega.Equal(pod.Spec.Containers[i].Name))
			}

			ginkgo.By(fmt.Sprintf("sidecarSet inject pod sidecar container, on in-place update done"))
		})

		ginkgo.It("sidecarSet re-inject pod sidecar container, on in-place update main container volumeMounts", func() {
			// create sidecarSet
			sidecarSet := tester.NewBaseSidecarSet(ns)
			sidecarSet.Labels = map[string]string{sidecarcontrol.LabelSidecarSetMode: sidecarcontrol.SidecarSetASI}
			sidecarSet.Annotations = map[string]string{
				sidecarcontrol.SidecarInjectOnInplaceUpdate: "true",
			}
			sidecarSet.Spec.InitContainers[0].Image = "reg.docker.alibaba-inc.com/busybox:latest"
			sidecarSet.Spec.Containers[0].Image = "reg.docker.alibaba-inc.com/nginx:latest"
			sidecarSet.Spec.Containers[1].Image = "reg.docker.alibaba-inc.com/busybox:latest"
			ginkgo.By(fmt.Sprintf("Creating SidecarSet %s", sidecarSet.Name))
			tester.CreateSidecarSet(sidecarSet)

			// create cloneset
			cloneset := tester.NewBaseCloneSet(ns)
			cloneset.Spec.Replicas = utilpointer.Int32Ptr(1)
			cloneset.Labels = map[string]string{sigmakruiseapi.LabelCloneSetMode: sigmakruiseapi.CloneSetASI}
			cloneset.Spec.UpdateStrategy = appsv1alpha1.CloneSetUpdateStrategy{
				Type: appsv1alpha1.InPlaceOnlyCloneSetUpdateStrategyType,
			}
			cloneset.Spec.Template.Spec.Containers[0].Env = []corev1.EnvVar{
				{
					Name:  "TEST_ENV",
					Value: "value-1",
				},
			}
			cloneset.Spec.Template.Spec.Containers[0].Image = "reg.docker.alibaba-inc.com/base/busybox:latest"
			cloneset.Spec.Template.Spec.Tolerations = newAsiTolerations()
			ginkgo.By(fmt.Sprintf("Creating CloneSet(%s.%s)", cloneset.Namespace, cloneset.Name))
			cloneset = tester.CreateCloneSet(cloneset)

			// get pods and have sidecar containers
			pods, err := tester.GetSelectorPods(cloneset.Namespace, cloneset.Spec.Selector)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			pod := pods[0]
			gomega.Expect(pod.Spec.Containers).To(gomega.HaveLen(len(cloneset.Spec.Template.Spec.Containers) + len(sidecarSet.Spec.Containers)))
			gomega.Expect(pod.Spec.InitContainers).To(gomega.HaveLen(1))
			exceptContainers := []string{"nginx-sidecar", "main", "busybox-sidecar"}
			for i, except := range exceptContainers {
				gomega.Expect(except).To(gomega.Equal(pod.Spec.Containers[i].Name))
			}

			// update cloneset main container envs, don't trigger upgrade sidecar container
			ginkgo.By(fmt.Sprintf("in-place update cloneSet(%s.%s) for envs", cloneset.Namespace, cloneset.Name))
			cloneset, _ = kc.AppsV1alpha1().CloneSets(cloneset.Namespace).Get(cloneset.Name, metav1.GetOptions{})
			cloneset.Spec.Template.Spec.Containers[0].Env = []corev1.EnvVar{
				{
					Name:  "TEST_ENV",
					Value: "value-2",
				},
			}
			_, err = kc.AppsV1alpha1().CloneSets(cloneset.Namespace).Update(cloneset)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			time.Sleep(time.Second * 10)
			// check pods and sidecar container
			ginkgo.By(fmt.Sprintf("check whether sidecar container have volumeMounts"))
			pods, err = tester.GetSelectorPods(cloneset.Namespace, cloneset.Spec.Selector)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			pod = pods[0]
			envVar := util.GetContainerEnvValue(&pod.Spec.Containers[1], "TEST_ENV")
			gomega.Expect(envVar).To(gomega.Equal("value-2"))
			gomega.Expect(pod.Status.ContainerStatuses[0].RestartCount).To(gomega.Equal(int32(0)))
			gomega.Expect(pod.Status.ContainerStatuses[1].RestartCount).To(gomega.Equal(int32(1)))
			gomega.Expect(pod.Status.ContainerStatuses[2].RestartCount).To(gomega.Equal(int32(0)))

			// update cloneset spec container volume and upgrade sidecar container
			ginkgo.By(fmt.Sprintf("in-place update cloneSet(%s.%s) for volumeMounts", cloneset.Namespace, cloneset.Name))
			//get latest version cloneset object
			cloneset, _ = kc.AppsV1alpha1().CloneSets(cloneset.Namespace).Get(cloneset.Name, metav1.GetOptions{})
			cloneset.Spec.Template.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{
				{
					Name:      "main-volume",
					MountPath: "/main-volume",
				},
			}
			cloneset.Spec.Template.Spec.Volumes = []corev1.Volume{
				{
					Name: "main-volume",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			}
			_, err = kc.AppsV1alpha1().CloneSets(cloneset.Namespace).Update(cloneset)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			time.Sleep(time.Second * 10)

			// check pods and sidecar container volumeMounts
			ginkgo.By(fmt.Sprintf("check whether sidecar container have volumeMounts"))
			pods, err = tester.GetSelectorPods(cloneset.Namespace, cloneset.Spec.Selector)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			pod = pods[0]
			exceptVolumeMounts := &corev1.VolumeMount{
				Name:      "main-volume",
				MountPath: "/main-volume",
			}
			volumeM := util.GetContainerVolumeMount(&pod.Spec.Containers[0], "/main-volume")
			gomega.Expect(volumeM).To(gomega.Equal(exceptVolumeMounts))
			gomega.Expect(pod.Status.ContainerStatuses[0].RestartCount).To(gomega.Equal(int32(1)))
			gomega.Expect(pod.Status.ContainerStatuses[1].RestartCount).To(gomega.Equal(int32(2)))
			gomega.Expect(pod.Status.ContainerStatuses[2].RestartCount).To(gomega.Equal(int32(0)))
			ginkgo.By(fmt.Sprintf("sidecarSet re-inject pod sidecar container, on in-place update main container volumeMounts done"))
		})

		ginkgo.It("sidecarSet re-inject pod sidecar container, on in-place update and newPods don't have sidecar container", func() {
			// create sidecarSet
			sidecarSet := tester.NewBaseSidecarSet(ns)
			sidecarSet.Spec.UpdateStrategy = appsv1alpha1.SidecarSetUpdateStrategy{
				Type: appsv1alpha1.NotUpdateSidecarSetStrategyType,
			}
			sidecarSet.Labels = map[string]string{sidecarcontrol.LabelSidecarSetMode: sidecarcontrol.SidecarSetASI}
			sidecarSet.Annotations = map[string]string{
				sidecarcontrol.SidecarInjectOnInplaceUpdate: "true",
			}
			sidecarSet.Spec.InitContainers[0].Image = "reg.docker.alibaba-inc.com/busybox:latest"
			sidecarSet.Spec.Containers[0].Image = "reg.docker.alibaba-inc.com/nginx:latest"
			sidecarSet.Spec.Containers[1].Image = "reg.docker.alibaba-inc.com/busybox:latest"
			ginkgo.By(fmt.Sprintf("Creating SidecarSet %s", sidecarSet.Name))
			tester.CreateSidecarSet(sidecarSet)

			// create cloneset
			cloneset := tester.NewBaseCloneSet(ns)
			cloneset.Spec.Replicas = utilpointer.Int32Ptr(1)
			cloneset.Labels = map[string]string{sigmakruiseapi.LabelCloneSetMode: sigmakruiseapi.CloneSetASI}
			cloneset.Spec.UpdateStrategy = appsv1alpha1.CloneSetUpdateStrategy{
				Type: appsv1alpha1.InPlaceOnlyCloneSetUpdateStrategyType,
			}
			cloneset.Spec.Template.Spec.Containers[0].Image = "reg.docker.alibaba-inc.com/base/busybox:latest"
			cloneset.Spec.Template.Spec.Tolerations = newAsiTolerations()
			ginkgo.By(fmt.Sprintf("Creating CloneSet(%s.%s)", cloneset.Namespace, cloneset.Name))
			cloneset = tester.CreateCloneSet(cloneset)

			// get pods and have sidecar containers
			pods, err := tester.GetSelectorPods(cloneset.Namespace, cloneset.Spec.Selector)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			pod := pods[0]
			gomega.Expect(pod.Spec.Containers).To(gomega.HaveLen(len(cloneset.Spec.Template.Spec.Containers) + len(sidecarSet.Spec.Containers)))
			gomega.Expect(pod.Spec.InitContainers).To(gomega.HaveLen(1))
			exceptContainers := []string{"nginx-sidecar", "main", "busybox-sidecar"}
			for i, except := range exceptContainers {
				gomega.Expect(except).To(gomega.Equal(pod.Spec.Containers[i].Name))
			}

			// update sidecarSet image
			newSidecarImage := "reg.docker.alibaba-inc.com/base/nginx:latest"
			ginkgo.By(fmt.Sprintf("Update sidecarSet(%s) sidecar container image(%s)", sidecarSet.Name, newSidecarImage))
			sidecarSet, _ = kc.AppsV1alpha1().SidecarSets().Get(sidecarSet.Name, metav1.GetOptions{})
			sidecarSet.Spec.Containers[0].Image = newSidecarImage
			tester.UpdateSidecarSet(sidecarSet)
			time.Sleep(time.Second)

			// update pods and newPods don't have sidecar containers
			ginkgo.By(fmt.Sprintf("Update pod(%s.%s) main container envs", pod.Namespace, pod.Name))
			newPod, err := c.CoreV1().Pods(pod.Namespace).Get(pod.Name, metav1.GetOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			newPod.Spec.Containers = newPod.Spec.Containers[1:2]
			newPod.Spec.Containers[0].Env = append(newPod.Spec.Containers[0].Env, corev1.EnvVar{
				Name:  "TEST_ENV",
				Value: "value-1",
			})
			newPod.Annotations[sigmak8sapi.AnnotationPodSpecHash] = fmt.Sprintf("%s-v2", ns)
			gomega.Expect(newPod.Spec.Containers).To(gomega.HaveLen(1))
			tester.UpdatePod(newPod)
			time.Sleep(time.Second * 2)

			// check pods and have old sidecar containers
			ginkgo.By(fmt.Sprintf("check newPod(%s.%s) have old sidecar container", pod.Namespace, pod.Name))
			newPod, err = c.CoreV1().Pods(newPod.Namespace).Get(newPod.Name, metav1.GetOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(newPod.Spec.Containers).To(gomega.HaveLen(len(cloneset.Spec.Template.Spec.Containers) + len(sidecarSet.Spec.Containers)))
			envVar := util.GetContainerEnvVar(&newPod.Spec.Containers[1], "TEST_ENV")
			gomega.Expect(envVar).NotTo(gomega.BeNil())
			gomega.Expect(newPod.Spec.Containers[0].Image).To(gomega.Equal("reg.docker.alibaba-inc.com/nginx:latest"))

			// update pods and newPods don't have sidecar containers
			ginkgo.By(fmt.Sprintf("Update pod(%s.%s) main container volumeMounts", pod.Namespace, pod.Name))
			newPod, err = c.CoreV1().Pods(pod.Namespace).Get(pod.Name, metav1.GetOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			newPod.Spec.Containers = newPod.Spec.Containers[1:2]
			newPod.Spec.Containers[0].VolumeMounts = append(newPod.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
				Name:      "main-volume",
				MountPath: "/main-volume",
			})
			newPod.Spec.Volumes = []corev1.Volume{
				{
					Name: "main-volume",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			}
			newPod.Annotations[sigmak8sapi.AnnotationPodSpecHash] = fmt.Sprintf("%s-v3", ns)
			gomega.Expect(newPod.Spec.Containers).To(gomega.HaveLen(1))
			tester.UpdatePod(newPod)
			time.Sleep(time.Second * 2)

			// check pods and have new sidecar containers
			ginkgo.By(fmt.Sprintf("check newPod(%s.%s) have new sidecar container", pod.Namespace, pod.Name))
			newPod, err = c.CoreV1().Pods(newPod.Namespace).Get(newPod.Name, metav1.GetOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(newPod.Spec.Containers).To(gomega.HaveLen(len(cloneset.Spec.Template.Spec.Containers) + len(sidecarSet.Spec.Containers)))
			vMounts := util.GetContainerVolumeMount(&newPod.Spec.Containers[1], "/main-volume")
			gomega.Expect(vMounts).NotTo(gomega.BeNil())
			gomega.Expect(newPod.Spec.Containers[0].Image).To(gomega.Equal(newSidecarImage))

			ginkgo.By(fmt.Sprintf("sidecarSet re-inject pod sidecar container, on in-place update and newPods don't have sidecar container done"))
		})
	})

	framework.KruiseDescribe("SidecarSet Upgrade functionality [SidecarSeUpgrade]", func() {

		ginkgo.AfterEach(func() {
			if ginkgo.CurrentGinkgoTestDescription().Failed {
				framework.DumpDebugInfo(c, ns)
			}
			framework.Logf("Deleting all SidecarSet in cluster")
			tester.DeleteSidecarSets()
			tester.DeleteDeployments(ns)
		})

		ginkgo.It("sidecarSet upgrade cold sidecar container image", func() {
			sidecarSetIn := tester.NewBaseSidecarSet(ns)
			sidecarSetIn.Labels = map[string]string{sidecarcontrol.LabelSidecarSetMode: sidecarcontrol.SidecarSetASI}
			sidecarSetIn.Annotations = map[string]string{
				sidecarcontrol.SidecarInjectOnInplaceUpdate: "true",
			}
			sidecarSetIn.Spec.Containers = sidecarSetIn.Spec.Containers[:1]
			sidecarSetIn.Spec.InitContainers[0].Image = "reg.docker.alibaba-inc.com/busybox:latest"
			sidecarSetIn.Spec.Containers[0].Image = "reg.docker.alibaba-inc.com/nginx:latest"
			sidecarSetIn.Spec.UpdateStrategy = appsv1alpha1.SidecarSetUpdateStrategy{
				Type: appsv1alpha1.RollingUpdateSidecarSetStrategyType,
				MaxUnavailable: &intstr.IntOrString{
					Type:   intstr.Int,
					IntVal: 2,
				},
			}
			ginkgo.By(fmt.Sprintf("Creating SidecarSet %s", sidecarSetIn.Name))
			sidecarSetIn = tester.CreateSidecarSet(sidecarSetIn)

			// create deployment
			deploymentIn := tester.NewBaseDeployment(ns)
			deploymentIn.Spec.Template.Spec.Containers[0].Image = "reg.docker.alibaba-inc.com/busybox:latest"
			deploymentIn.Spec.Replicas = utilpointer.Int32Ptr(2)
			deploymentIn.Spec.Template.Spec.Tolerations = newAsiTolerations()
			ginkgo.By(fmt.Sprintf("Creating Deployment(%s.%s)", deploymentIn.Namespace, deploymentIn.Name))
			tester.CreateDeployment(deploymentIn)

			// update sidecarSet sidecar container
			ginkgo.By(fmt.Sprintf("Update SidecarSet %s sidecar image", sidecarSetIn.Name))
			sidecarSetIn, _ = kc.AppsV1alpha1().SidecarSets().Get(sidecarSetIn.Name, metav1.GetOptions{})
			sidecarSetIn.Spec.Containers[0].Image = "reg.docker.alibaba-inc.com/base/nginx:latest"
			tester.UpdateSidecarSet(sidecarSetIn)
			time.Sleep(time.Second * 5)
			except := &appsv1alpha1.SidecarSetStatus{
				MatchedPods:      2,
				UpdatedPods:      2,
				UpdatedReadyPods: 2,
				ReadyPods:        2,
			}
			tester.WaitForSidecarSetUpgradeComplete(sidecarSetIn, except)
			// get pods
			pods, err := tester.GetSelectorPods(deploymentIn.Namespace, deploymentIn.Spec.Selector)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			for _, pod := range pods {
				sidecarContainer := pod.Spec.Containers[0]
				gomega.Expect(sidecarContainer.Image).To(gomega.Equal("reg.docker.alibaba-inc.com/base/nginx:latest"))
			}

			ginkgo.By(fmt.Sprintf("sidecarSet upgrade cold sidecar container image done"))
		})

		ginkgo.It("sidecarSet upgrade cold sidecar container failed image, and only update one pod", func() {
			sidecarSetIn := tester.NewBaseSidecarSet(ns)
			sidecarSetIn.Labels = map[string]string{sidecarcontrol.LabelSidecarSetMode: sidecarcontrol.SidecarSetASI}
			sidecarSetIn.Annotations = map[string]string{
				sidecarcontrol.SidecarInjectOnInplaceUpdate: "true",
			}
			sidecarSetIn.Spec.Containers = sidecarSetIn.Spec.Containers[:1]
			sidecarSetIn.Spec.InitContainers[0].Image = "reg.docker.alibaba-inc.com/busybox:latest"
			sidecarSetIn.Spec.Containers[0].Image = "reg.docker.alibaba-inc.com/nginx:latest"
			sidecarSetIn.Spec.UpdateStrategy = appsv1alpha1.SidecarSetUpdateStrategy{
				Type: appsv1alpha1.RollingUpdateSidecarSetStrategyType,
			}
			ginkgo.By(fmt.Sprintf("Creating SidecarSet %s", sidecarSetIn.Name))
			sidecarSetIn = tester.CreateSidecarSet(sidecarSetIn)

			// create deployment
			deploymentIn := tester.NewBaseDeployment(ns)
			deploymentIn.Spec.Replicas = utilpointer.Int32Ptr(2)
			deploymentIn.Spec.Template.Spec.Containers[0].Image = "reg.docker.alibaba-inc.com/busybox:latest"
			deploymentIn.Spec.Template.Spec.Tolerations = newAsiTolerations()
			ginkgo.By(fmt.Sprintf("Creating Deployment(%s.%s)", deploymentIn.Namespace, deploymentIn.Name))
			tester.CreateDeployment(deploymentIn)

			// update sidecarSet sidecar container failed image
			ginkgo.By(fmt.Sprintf("update sidecarSet(%s) failed image", sidecarSetIn.Name))
			sidecarSetIn, _ = kc.AppsV1alpha1().SidecarSets().Get(sidecarSetIn.Name, metav1.GetOptions{})
			sidecarSetIn.Spec.Containers[0].Image = "reg.docker.alibaba-inc.com/nginx:failed"
			tester.UpdateSidecarSet(sidecarSetIn)
			time.Sleep(time.Second * 30)
			except := &appsv1alpha1.SidecarSetStatus{
				MatchedPods:      2,
				UpdatedPods:      1,
				UpdatedReadyPods: 0,
				ReadyPods:        1,
			}
			tester.WaitForSidecarSetUpgradeComplete(sidecarSetIn, except)

			// update sidecarSet sidecar container success image
			ginkgo.By(fmt.Sprintf("update sidecarSet(%s) success image", sidecarSetIn.Name))
			sidecarSetIn.Spec.Containers[0].Image = "reg.docker.alibaba-inc.com/base/nginx:latest"
			tester.UpdateSidecarSet(sidecarSetIn)
			time.Sleep(time.Second * 5)
			except = &appsv1alpha1.SidecarSetStatus{
				MatchedPods:      2,
				UpdatedPods:      2,
				UpdatedReadyPods: 2,
				ReadyPods:        2,
			}
			tester.WaitForSidecarSetUpgradeComplete(sidecarSetIn, except)
			ginkgo.By(fmt.Sprintf("sidecarSet upgrade cold sidecar container failed image, and only update one pod done"))
		})

		ginkgo.It("sidecarSet upgrade cold sidecar container image, and paused", func() {
			sidecarSetIn := tester.NewBaseSidecarSet(ns)
			sidecarSetIn.Labels = map[string]string{sidecarcontrol.LabelSidecarSetMode: sidecarcontrol.SidecarSetASI}
			sidecarSetIn.Annotations = map[string]string{
				sidecarcontrol.SidecarInjectOnInplaceUpdate: "true",
			}
			sidecarSetIn.Spec.Containers = sidecarSetIn.Spec.Containers[:1]
			sidecarSetIn.Spec.InitContainers[0].Image = "reg.docker.alibaba-inc.com/busybox:latest"
			sidecarSetIn.Spec.Containers[0].Image = "reg.docker.alibaba-inc.com/nginx:latest"
			sidecarSetIn.Spec.UpdateStrategy = appsv1alpha1.SidecarSetUpdateStrategy{
				Type: appsv1alpha1.RollingUpdateSidecarSetStrategyType,
			}
			ginkgo.By(fmt.Sprintf("Creating SidecarSet %s", sidecarSetIn.Name))
			sidecarSetIn = tester.CreateSidecarSet(sidecarSetIn)

			// create deployment
			deploymentIn := tester.NewBaseDeployment(ns)
			deploymentIn.Spec.Replicas = utilpointer.Int32Ptr(2)
			deploymentIn.Spec.Template.Spec.Containers[0].Image = "reg.docker.alibaba-inc.com/busybox:latest"
			deploymentIn.Spec.Template.Spec.Tolerations = newAsiTolerations()
			ginkgo.By(fmt.Sprintf("Creating Deployment(%s.%s)", deploymentIn.Namespace, deploymentIn.Name))
			tester.CreateDeployment(deploymentIn)

			// update sidecarSet sidecar container
			ginkgo.By(fmt.Sprintf("update sidecarSet(%s) sidecar image, and paused", sidecarSetIn.Name))
			sidecarSetIn.Spec.Containers[0].Image = "reg.docker.alibaba-inc.com/base/nginx:latest"
			tester.UpdateSidecarSet(sidecarSetIn)
			time.Sleep(time.Second * 5)
			// paused
			sidecarSetIn.Spec.UpdateStrategy.Paused = true
			tester.UpdateSidecarSet(sidecarSetIn)

			time.Sleep(time.Second * 30)
			except := &appsv1alpha1.SidecarSetStatus{
				MatchedPods:      2,
				UpdatedPods:      1,
				UpdatedReadyPods: 1,
				ReadyPods:        2,
			}
			tester.WaitForSidecarSetUpgradeComplete(sidecarSetIn, except)

			// paused = false, continue update pods
			sidecarSetIn.Spec.UpdateStrategy.Paused = false
			tester.UpdateSidecarSet(sidecarSetIn)
			except = &appsv1alpha1.SidecarSetStatus{
				MatchedPods:      2,
				UpdatedPods:      2,
				UpdatedReadyPods: 2,
				ReadyPods:        2,
			}
			tester.WaitForSidecarSetUpgradeComplete(sidecarSetIn, except)
			ginkgo.By(fmt.Sprintf("sidecarSet upgrade cold sidecar container image, and paused done"))
		})

		ginkgo.It("sidecarSet upgrade cold sidecar container image, and selector", func() {
			sidecarSetIn := tester.NewBaseSidecarSet(ns)
			sidecarSetIn.Labels = map[string]string{sidecarcontrol.LabelSidecarSetMode: sidecarcontrol.SidecarSetASI}
			sidecarSetIn.Annotations = map[string]string{
				sidecarcontrol.SidecarInjectOnInplaceUpdate: "true",
			}
			sidecarSetIn.Spec.Containers = sidecarSetIn.Spec.Containers[:1]
			sidecarSetIn.Spec.InitContainers[0].Image = "reg.docker.alibaba-inc.com/busybox:latest"
			sidecarSetIn.Spec.Containers[0].Image = "reg.docker.alibaba-inc.com/nginx:latest"
			sidecarSetIn.Spec.UpdateStrategy = appsv1alpha1.SidecarSetUpdateStrategy{
				Type: appsv1alpha1.RollingUpdateSidecarSetStrategyType,
			}
			ginkgo.By(fmt.Sprintf("Creating SidecarSet %s", sidecarSetIn.Name))
			sidecarSetIn = tester.CreateSidecarSet(sidecarSetIn)

			// create deployment
			deploymentIn := tester.NewBaseDeployment(ns)
			deploymentIn.Spec.Replicas = utilpointer.Int32Ptr(2)
			deploymentIn.Spec.Template.Spec.Containers[0].Image = "reg.docker.alibaba-inc.com/busybox:latest"
			deploymentIn.Spec.Template.Spec.Tolerations = newAsiTolerations()
			ginkgo.By(fmt.Sprintf("Creating Deployment(%s.%s)", deploymentIn.Namespace, deploymentIn.Name))
			tester.CreateDeployment(deploymentIn)

			// update pod[0] labels[canary.release] = true
			pods, err := tester.GetSelectorPods(deploymentIn.Namespace, deploymentIn.Spec.Selector)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(pods).To(gomega.HaveLen(int(*deploymentIn.Spec.Replicas)))
			canaryPod := pods[0]
			canaryPod.Labels["canary.release"] = "true"
			ginkgo.By(fmt.Sprintf("update pod(%s.%s) Labels[canary.release] = true", canaryPod.Namespace, canaryPod.Name))
			tester.UpdatePod(&canaryPod)
			time.Sleep(time.Second)

			ginkgo.By(fmt.Sprintf("update SidecarSet %s sidecar container image", sidecarSetIn.Name))
			// update sidecarSet sidecar container
			sidecarSetIn.Spec.Containers[0].Image = "reg.docker.alibaba-inc.com/base/nginx:latest"
			// update sidecarSet selector
			sidecarSetIn.Spec.UpdateStrategy.Selector = &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"canary.release": "true",
				},
			}
			tester.UpdateSidecarSet(sidecarSetIn)

			ginkgo.By(fmt.Sprintf("waiting for 30 seconds, and check SidecarSet and pods"))
			time.Sleep(time.Second * 30)
			except := &appsv1alpha1.SidecarSetStatus{
				MatchedPods:      2,
				UpdatedPods:      1,
				UpdatedReadyPods: 1,
				ReadyPods:        2,
			}
			tester.WaitForSidecarSetUpgradeComplete(sidecarSetIn, except)
			// check pod image
			pods, err = tester.GetSelectorPods(deploymentIn.Namespace, deploymentIn.Spec.Selector)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(pods).To(gomega.HaveLen(int(*deploymentIn.Spec.Replicas)))
			for _, pod := range pods {
				if _, ok := pod.Labels["canary.release"]; ok {
					sidecarContainer := pod.Spec.Containers[0]
					gomega.Expect(sidecarContainer.Image).To(gomega.Equal("reg.docker.alibaba-inc.com/base/nginx:latest"))
				} else {
					sidecarContainer := pod.Spec.Containers[0]
					gomega.Expect(sidecarContainer.Image).To(gomega.Equal("reg.docker.alibaba-inc.com/nginx:latest"))
				}
			}

			// update sidecarSet selector == nil, and update all pods
			ginkgo.By(fmt.Sprintf("update sidecarSet %s selector == nil", sidecarSetIn.Name))
			sidecarSetIn.Spec.UpdateStrategy.Selector = nil
			tester.UpdateSidecarSet(sidecarSetIn)

			ginkgo.By(fmt.Sprintf("waiting for 5 seconds, and update all pods"))
			time.Sleep(time.Second * 5)
			except = &appsv1alpha1.SidecarSetStatus{
				MatchedPods:      2,
				UpdatedPods:      2,
				UpdatedReadyPods: 2,
				ReadyPods:        2,
			}
			tester.WaitForSidecarSetUpgradeComplete(sidecarSetIn, except)
			ginkgo.By(fmt.Sprintf("sidecarSet upgrade cold sidecar container image, and selector done"))
		})

		ginkgo.It("sidecarSet upgrade cold sidecar container image, and partition", func() {
			sidecarSetIn := tester.NewBaseSidecarSet(ns)
			sidecarSetIn.Labels = map[string]string{sidecarcontrol.LabelSidecarSetMode: sidecarcontrol.SidecarSetASI}
			sidecarSetIn.Annotations = map[string]string{
				sidecarcontrol.SidecarInjectOnInplaceUpdate: "true",
			}
			sidecarSetIn.Spec.Containers = sidecarSetIn.Spec.Containers[:1]
			sidecarSetIn.Spec.InitContainers[0].Image = "reg.docker.alibaba-inc.com/busybox:latest"
			sidecarSetIn.Spec.Containers[0].Image = "reg.docker.alibaba-inc.com/nginx:latest"
			sidecarSetIn.Spec.UpdateStrategy = appsv1alpha1.SidecarSetUpdateStrategy{
				Type: appsv1alpha1.RollingUpdateSidecarSetStrategyType,
			}
			ginkgo.By(fmt.Sprintf("Creating SidecarSet %s", sidecarSetIn.Name))
			sidecarSetIn = tester.CreateSidecarSet(sidecarSetIn)

			// create deployment
			deploymentIn := tester.NewBaseDeployment(ns)
			deploymentIn.Spec.Replicas = utilpointer.Int32Ptr(2)
			deploymentIn.Spec.Template.Spec.Containers[0].Image = "reg.docker.alibaba-inc.com/busybox:latest"
			deploymentIn.Spec.Template.Spec.Tolerations = newAsiTolerations()
			ginkgo.By(fmt.Sprintf("Creating Deployment(%s.%s)", deploymentIn.Namespace, deploymentIn.Name))
			tester.CreateDeployment(deploymentIn)

			// update sidecarSet sidecar container
			ginkgo.By(fmt.Sprintf("update SidecarSet %s sidecar container image, and partition", sidecarSetIn.Name))
			sidecarSetIn.Spec.Containers[0].Image = "reg.docker.alibaba-inc.com/base/nginx:latest"
			// update sidecarSet partition
			sidecarSetIn.Spec.UpdateStrategy.Partition = &intstr.IntOrString{
				Type:   intstr.String,
				StrVal: "50%",
			}
			tester.UpdateSidecarSet(sidecarSetIn)
			except := &appsv1alpha1.SidecarSetStatus{
				MatchedPods:      2,
				UpdatedPods:      1,
				UpdatedReadyPods: 1,
				ReadyPods:        2,
			}
			time.Sleep(time.Second * 30)
			tester.WaitForSidecarSetUpgradeComplete(sidecarSetIn, except)

			// update sidecarSet partition, update all pods
			ginkgo.By(fmt.Sprintf("update SidecarSet %s partition = nil, and all pods updated", sidecarSetIn.Name))
			sidecarSetIn.Spec.UpdateStrategy.Partition = nil
			tester.UpdateSidecarSet(sidecarSetIn)
			except = &appsv1alpha1.SidecarSetStatus{
				MatchedPods:      2,
				UpdatedPods:      2,
				UpdatedReadyPods: 2,
				ReadyPods:        2,
			}
			time.Sleep(time.Second * 5)
			tester.WaitForSidecarSetUpgradeComplete(sidecarSetIn, except)

			ginkgo.By(fmt.Sprintf("sidecarSet upgrade cold sidecar container image, and partition done"))
		})

		ginkgo.It("sidecarSet upgrade cold sidecar container image, and maxUnavailable", func() {
			sidecarSetIn := tester.NewBaseSidecarSet(ns)
			sidecarSetIn.Labels = map[string]string{sidecarcontrol.LabelSidecarSetMode: sidecarcontrol.SidecarSetASI}
			sidecarSetIn.Annotations = map[string]string{
				sidecarcontrol.SidecarInjectOnInplaceUpdate: "true",
			}
			sidecarSetIn.Spec.Containers = sidecarSetIn.Spec.Containers[:1]
			sidecarSetIn.Spec.InitContainers[0].Image = "reg.docker.alibaba-inc.com/busybox:latest"
			sidecarSetIn.Spec.Containers[0].Image = "reg.docker.alibaba-inc.com/nginx:latest"
			sidecarSetIn.Spec.UpdateStrategy = appsv1alpha1.SidecarSetUpdateStrategy{
				Type: appsv1alpha1.RollingUpdateSidecarSetStrategyType,
			}
			ginkgo.By(fmt.Sprintf("Creating SidecarSet %s", sidecarSetIn.Name))
			sidecarSetIn = tester.CreateSidecarSet(sidecarSetIn)

			// create deployment
			deploymentIn := tester.NewBaseDeployment(ns)
			deploymentIn.Spec.Replicas = utilpointer.Int32Ptr(2)
			deploymentIn.Spec.Template.Spec.Containers[0].Image = "reg.docker.alibaba-inc.com/busybox:latest"
			deploymentIn.Spec.Template.Spec.Tolerations = newAsiTolerations()
			ginkgo.By(fmt.Sprintf("Creating Deployment(%s.%s)", deploymentIn.Namespace, deploymentIn.Name))
			tester.CreateDeployment(deploymentIn)

			// update sidecarSet sidecar container
			ginkgo.By(fmt.Sprintf("update SidecarSet %s sidecar container failed image, and MaxUnavailable", sidecarSetIn.Name))
			sidecarSetIn.Spec.Containers[0].Image = "reg.docker.alibaba-inc.com/nginx:failed"
			// update sidecarSet MaxUnavailable
			sidecarSetIn.Spec.UpdateStrategy.MaxUnavailable = &intstr.IntOrString{
				Type:   intstr.String,
				StrVal: "50%",
			}
			tester.UpdateSidecarSet(sidecarSetIn)
			except := &appsv1alpha1.SidecarSetStatus{
				MatchedPods:      2,
				UpdatedPods:      1,
				UpdatedReadyPods: 0,
				ReadyPods:        1,
			}
			time.Sleep(time.Second * 30)
			tester.WaitForSidecarSetUpgradeComplete(sidecarSetIn, except)

			// update sidecarSet sidecar container
			ginkgo.By(fmt.Sprintf("update SidecarSet %s sidecar container success image", sidecarSetIn.Name))
			sidecarSetIn.Spec.Containers[0].Image = "reg.docker.alibaba-inc.com/base/nginx:latest"
			tester.UpdateSidecarSet(sidecarSetIn)
			except = &appsv1alpha1.SidecarSetStatus{
				MatchedPods:      2,
				UpdatedPods:      2,
				UpdatedReadyPods: 2,
				ReadyPods:        2,
			}
			time.Sleep(time.Second * 5)
			tester.WaitForSidecarSetUpgradeComplete(sidecarSetIn, except)
			ginkgo.By(fmt.Sprintf("sidecarSet upgrade cold sidecar container image, and maxUnavailable done"))
		})
	})

	framework.KruiseDescribe("SidecarSet HotUpgrade functionality [SidecarSetHotUpgrade]", func() {

		ginkgo.AfterEach(func() {
			if ginkgo.CurrentGinkgoTestDescription().Failed {
				framework.DumpDebugInfo(c, ns)
			}
			framework.Logf("Deleting all SidecarSet in cluster")
			tester.DeleteSidecarSets()
			tester.DeleteDeployments(ns)
		})

		ginkgo.It("sidecarSet inject pod hot upgrade sidecar container", func() {
			// create sidecarSet
			sidecarSetIn := tester.NewBaseSidecarSet(ns)
			sidecarSetIn.Labels = map[string]string{sidecarcontrol.LabelSidecarSetMode: sidecarcontrol.SidecarSetASI}
			sidecarSetIn.Annotations = map[string]string{
				sidecarcontrol.SidecarInjectOnInplaceUpdate: "true",
			}
			sidecarSetIn.Spec.Containers = sidecarSetIn.Spec.Containers[:1]
			sidecarSetIn.Spec.InitContainers[0].Image = "reg.docker.alibaba-inc.com/busybox:latest"
			sidecarSetIn.Spec.Containers[0].Image = "reg.docker.alibaba-inc.com/nginx:latest"
			sidecarSetIn.Spec.Containers[0].UpgradeStrategy = appsv1alpha1.SidecarContainerUpgradeStrategy{
				UpgradeType:          appsv1alpha1.SidecarContainerHotUpgrade,
				HotUpgradeEmptyImage: "reg.docker.alibaba-inc.com/busybox:latest",
			}
			ginkgo.By(fmt.Sprintf("Creating SidecarSet %s", sidecarSetIn.Name))
			tester.CreateSidecarSet(sidecarSetIn)

			// create deployment
			deploymentIn := tester.NewBaseDeployment(ns)
			deploymentIn.Spec.Replicas = utilpointer.Int32Ptr(2)
			deploymentIn.Spec.Template.Spec.Containers[0].Image = "reg.docker.alibaba-inc.com/busybox:latest"
			deploymentIn.Spec.Template.Spec.Tolerations = newAsiTolerations()
			ginkgo.By(fmt.Sprintf("Creating Deployment(%s.%s)", deploymentIn.Namespace, deploymentIn.Name))
			tester.CreateDeployment(deploymentIn)

			// get pods
			pods, err := tester.GetSelectorPods(deploymentIn.Namespace, deploymentIn.Spec.Selector)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			for _, pod := range pods {
				gomega.Expect(pod.Spec.Containers).To(gomega.HaveLen(len(sidecarSetIn.Spec.Containers) + len(deploymentIn.Spec.Template.Spec.Containers) + 1))
				// except sidecar container -> image
				exceptContainer := map[string]string{
					"nginx-sidecar-1": "reg.docker.alibaba-inc.com/nginx:latest",
					"nginx-sidecar-2": "reg.docker.alibaba-inc.com/busybox:latest",
				}
				for sidecar, image := range exceptContainer {
					sidecarContainer := util.GetContainer(sidecar, &pod)
					gomega.Expect(sidecarContainer).ShouldNot(gomega.BeNil())
					gomega.Expect(sidecarContainer.Image).To(gomega.Equal(image))
				}
			}
			ginkgo.By(fmt.Sprintf("sidecarSet inject pod hot upgrade sidecar container done"))
		})

		ginkgo.It("sidecarSet upgrade hot sidecar container image", func() {
			// create sidecarSet
			sidecarSetIn := tester.NewBaseSidecarSet(ns)
			sidecarSetIn.Labels = map[string]string{sidecarcontrol.LabelSidecarSetMode: sidecarcontrol.SidecarSetASI}
			sidecarSetIn.Annotations = map[string]string{
				sidecarcontrol.SidecarInjectOnInplaceUpdate: "true",
			}
			sidecarSetIn.Spec.Containers = sidecarSetIn.Spec.Containers[:1]
			sidecarSetIn.Spec.InitContainers[0].Image = "reg.docker.alibaba-inc.com/busybox:latest"
			sidecarSetIn.Spec.Containers[0].Image = "reg.docker.alibaba-inc.com/nginx:latest"
			sidecarSetIn.Spec.Containers[0].UpgradeStrategy = appsv1alpha1.SidecarContainerUpgradeStrategy{
				UpgradeType:          appsv1alpha1.SidecarContainerHotUpgrade,
				HotUpgradeEmptyImage: "reg.docker.alibaba-inc.com/busybox:latest",
			}
			ginkgo.By(fmt.Sprintf("Creating SidecarSet %s", sidecarSetIn.Name))
			tester.CreateSidecarSet(sidecarSetIn)

			// create deployment
			deploymentIn := tester.NewBaseDeployment(ns)
			deploymentIn.Spec.Replicas = utilpointer.Int32Ptr(1)
			deploymentIn.Spec.Template.Spec.Containers[0].Image = "reg.docker.alibaba-inc.com/busybox:latest"
			deploymentIn.Spec.Template.Spec.Tolerations = newAsiTolerations()
			ginkgo.By(fmt.Sprintf("Creating Deployment(%s.%s)", deploymentIn.Namespace, deploymentIn.Name))
			tester.CreateDeployment(deploymentIn)

			// check pod image and annotations
			pods, err := tester.GetSelectorPods(deploymentIn.Namespace, deploymentIn.Spec.Selector)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(pods).To(gomega.HaveLen(int(*deploymentIn.Spec.Replicas)))
			podIn := &pods[0]
			workSidecarContainer := util.GetContainer(sidecarcontrol.GetPodHotUpgradeInfoInAnnotations(podIn)["nginx-sidecar"], podIn)
			gomega.Expect(workSidecarContainer.Name).To(gomega.Equal("nginx-sidecar-1"))
			gomega.Expect(workSidecarContainer.Image).To(gomega.Equal("reg.docker.alibaba-inc.com/nginx:latest"))
			_, emptyContainer := sidecarcontrol.GetPodHotUpgradeContainers("nginx-sidecar", podIn)
			emptySidecarContainer := util.GetContainer(emptyContainer, podIn)
			gomega.Expect(emptySidecarContainer.Name).To(gomega.Equal("nginx-sidecar-2"))
			gomega.Expect(emptySidecarContainer.Image).To(gomega.Equal("reg.docker.alibaba-inc.com/busybox:latest"))

			// update sidecarSet sidecar container
			ginkgo.By(fmt.Sprintf("update SidecarSet %s sidecar container image", sidecarSetIn.Name))
			sidecarSetIn.Spec.Containers[0].Image = "reg.docker.alibaba-inc.com/base/nginx:latest"
			tester.UpdateSidecarSet(sidecarSetIn)
			except := &appsv1alpha1.SidecarSetStatus{
				MatchedPods:      1,
				UpdatedPods:      1,
				UpdatedReadyPods: 1,
				ReadyPods:        1,
			}
			time.Sleep(time.Second * 10)
			tester.WaitForSidecarSetUpgradeComplete(sidecarSetIn, except)

			// check pod image and annotations
			pods, err = tester.GetSelectorPods(deploymentIn.Namespace, deploymentIn.Spec.Selector)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(pods).To(gomega.HaveLen(int(*deploymentIn.Spec.Replicas)))
			podIn = &pods[0]
			workSidecarContainer = util.GetContainer(sidecarcontrol.GetPodHotUpgradeInfoInAnnotations(podIn)["nginx-sidecar"], podIn)
			gomega.Expect(workSidecarContainer.Name).To(gomega.Equal("nginx-sidecar-2"))
			gomega.Expect(workSidecarContainer.Image).To(gomega.Equal("reg.docker.alibaba-inc.com/base/nginx:latest"))
			_, emptyContainer = sidecarcontrol.GetPodHotUpgradeContainers("nginx-sidecar", podIn)
			emptySidecarContainer = util.GetContainer(emptyContainer, podIn)
			gomega.Expect(emptySidecarContainer.Name).To(gomega.Equal("nginx-sidecar-1"))
			gomega.Expect(emptySidecarContainer.Image).To(gomega.Equal("reg.docker.alibaba-inc.com/busybox:latest"))

			//update sidecarSet sidecar container again
			ginkgo.By(fmt.Sprintf("update SidecarSet %s sidecar container image again", sidecarSetIn.Name))
			sidecarSetIn.Spec.Containers[0].Image = "reg.docker.alibaba-inc.com/nginx:latest"
			tester.UpdateSidecarSet(sidecarSetIn)
			except = &appsv1alpha1.SidecarSetStatus{
				MatchedPods:      1,
				UpdatedPods:      1,
				UpdatedReadyPods: 1,
				ReadyPods:        1,
			}
			time.Sleep(time.Second * 10)
			tester.WaitForSidecarSetUpgradeComplete(sidecarSetIn, except)
			// check pod image and annotations
			pods, err = tester.GetSelectorPods(deploymentIn.Namespace, deploymentIn.Spec.Selector)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(pods).To(gomega.HaveLen(int(*deploymentIn.Spec.Replicas)))
			podIn = &pods[0]
			workSidecarContainer = util.GetContainer(sidecarcontrol.GetPodHotUpgradeInfoInAnnotations(podIn)["nginx-sidecar"], podIn)
			gomega.Expect(workSidecarContainer.Name).To(gomega.Equal("nginx-sidecar-1"))
			gomega.Expect(workSidecarContainer.Image).To(gomega.Equal("reg.docker.alibaba-inc.com/nginx:latest"))
			_, emptyContainer = sidecarcontrol.GetPodHotUpgradeContainers("nginx-sidecar", podIn)
			emptySidecarContainer = util.GetContainer(emptyContainer, podIn)
			gomega.Expect(emptySidecarContainer.Name).To(gomega.Equal("nginx-sidecar-2"))
			gomega.Expect(emptySidecarContainer.Image).To(gomega.Equal("reg.docker.alibaba-inc.com/busybox:latest"))
			ginkgo.By(fmt.Sprintf("sidecarSet upgrade hot sidecar container image done"))
		})

		ginkgo.It("sidecarSet upgrade hot sidecar container failed image", func() {
			// create sidecarSet
			sidecarSetIn := tester.NewBaseSidecarSet(ns)
			sidecarSetIn.Labels = map[string]string{sidecarcontrol.LabelSidecarSetMode: sidecarcontrol.SidecarSetASI}
			sidecarSetIn.Annotations = map[string]string{
				sidecarcontrol.SidecarInjectOnInplaceUpdate: "true",
			}
			sidecarSetIn.Spec.Containers = sidecarSetIn.Spec.Containers[:1]
			sidecarSetIn.Spec.InitContainers[0].Image = "reg.docker.alibaba-inc.com/busybox:latest"
			sidecarSetIn.Spec.Containers[0].Image = "reg.docker.alibaba-inc.com/nginx:latest"
			sidecarSetIn.Spec.Containers[0].UpgradeStrategy = appsv1alpha1.SidecarContainerUpgradeStrategy{
				UpgradeType:          appsv1alpha1.SidecarContainerHotUpgrade,
				HotUpgradeEmptyImage: "reg.docker.alibaba-inc.com/busybox:latest",
			}
			ginkgo.By(fmt.Sprintf("Creating SidecarSet %s", sidecarSetIn.Name))
			tester.CreateSidecarSet(sidecarSetIn)

			// create deployment
			deploymentIn := tester.NewBaseDeployment(ns)
			deploymentIn.Spec.Replicas = utilpointer.Int32Ptr(2)
			deploymentIn.Spec.Template.Spec.Containers[0].Image = "reg.docker.alibaba-inc.com/busybox:latest"
			deploymentIn.Spec.Template.Spec.Tolerations = newAsiTolerations()
			ginkgo.By(fmt.Sprintf("Creating Deployment(%s.%s)", deploymentIn.Namespace, deploymentIn.Name))
			tester.CreateDeployment(deploymentIn)

			// check pod image and annotations
			pods, err := tester.GetSelectorPods(deploymentIn.Namespace, deploymentIn.Spec.Selector)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(pods).To(gomega.HaveLen(int(*deploymentIn.Spec.Replicas)))
			podIn := &pods[0]
			workSidecarContainer := util.GetContainer(sidecarcontrol.GetPodHotUpgradeInfoInAnnotations(podIn)["nginx-sidecar"], podIn)
			gomega.Expect(workSidecarContainer.Name).To(gomega.Equal("nginx-sidecar-1"))
			gomega.Expect(workSidecarContainer.Image).To(gomega.Equal("reg.docker.alibaba-inc.com/nginx:latest"))
			_, emptyContainer := sidecarcontrol.GetPodHotUpgradeContainers("nginx-sidecar", podIn)
			emptySidecarContainer := util.GetContainer(emptyContainer, podIn)
			gomega.Expect(emptySidecarContainer.Name).To(gomega.Equal("nginx-sidecar-2"))
			gomega.Expect(emptySidecarContainer.Image).To(gomega.Equal("reg.docker.alibaba-inc.com/busybox:latest"))

			// update sidecarSet sidecar container failed image
			ginkgo.By(fmt.Sprintf("Update SidecarSet(%s) with failed image", sidecarSetIn.Name))
			sidecarSetIn.Spec.Containers[0].Image = "reg.docker.alibaba-inc.com/nginx:failed"
			tester.UpdateSidecarSet(sidecarSetIn)
			except := &appsv1alpha1.SidecarSetStatus{
				MatchedPods:      2,
				UpdatedPods:      1,
				UpdatedReadyPods: 0,
				ReadyPods:        1,
			}
			time.Sleep(time.Second * 30)
			tester.WaitForSidecarSetUpgradeComplete(sidecarSetIn, except)

			ginkgo.By(fmt.Sprintf("Update SidecarSet(%s) with success image", sidecarSetIn.Name))
			//update sidecarSet sidecar container again, and success image
			sidecarSetIn.Spec.Containers[0].Image = "reg.docker.alibaba-inc.com/base/nginx:latest"
			tester.UpdateSidecarSet(sidecarSetIn)
			except = &appsv1alpha1.SidecarSetStatus{
				MatchedPods:      2,
				UpdatedPods:      2,
				UpdatedReadyPods: 2,
				ReadyPods:        2,
			}
			time.Sleep(time.Second * 10)
			tester.WaitForSidecarSetUpgradeComplete(sidecarSetIn, except)
			// check pod image and annotations
			pods, err = tester.GetSelectorPods(deploymentIn.Namespace, deploymentIn.Spec.Selector)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(pods).To(gomega.HaveLen(int(*deploymentIn.Spec.Replicas)))
			// pod[0]
			podIn1 := &pods[0]
			workSidecarContainer = util.GetContainer(sidecarcontrol.GetPodHotUpgradeInfoInAnnotations(podIn1)["nginx-sidecar"], podIn1)
			gomega.Expect(workSidecarContainer.Name).To(gomega.Equal("nginx-sidecar-2"))
			gomega.Expect(workSidecarContainer.Image).To(gomega.Equal("reg.docker.alibaba-inc.com/base/nginx:latest"))
			_, emptyContainer = sidecarcontrol.GetPodHotUpgradeContainers("nginx-sidecar", podIn1)
			emptySidecarContainer = util.GetContainer(emptyContainer, podIn1)
			gomega.Expect(emptySidecarContainer.Name).To(gomega.Equal("nginx-sidecar-1"))
			gomega.Expect(emptySidecarContainer.Image).To(gomega.Equal("reg.docker.alibaba-inc.com/busybox:latest"))
			// pod[1]
			podIn2 := &pods[1]
			workSidecarContainer = util.GetContainer(sidecarcontrol.GetPodHotUpgradeInfoInAnnotations(podIn2)["nginx-sidecar"], podIn2)
			gomega.Expect(workSidecarContainer.Name).To(gomega.Equal("nginx-sidecar-2"))
			gomega.Expect(workSidecarContainer.Image).To(gomega.Equal("reg.docker.alibaba-inc.com/base/nginx:latest"))
			_, emptyContainer = sidecarcontrol.GetPodHotUpgradeContainers("nginx-sidecar", podIn2)
			emptySidecarContainer = util.GetContainer(emptyContainer, podIn2)
			gomega.Expect(emptySidecarContainer.Name).To(gomega.Equal("nginx-sidecar-1"))
			gomega.Expect(emptySidecarContainer.Image).To(gomega.Equal("reg.docker.alibaba-inc.com/busybox:latest"))

			ginkgo.By(fmt.Sprintf("sidecarSet upgrade hot sidecar container failed image done"))
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

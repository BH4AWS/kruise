package enhancedlivenessprobe

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	alpha1 "github.com/openkruise/kruise/apis/apps/v1alpha1"
	appsv1alpha1 "github.com/openkruise/kruise/apis/apps/v1alpha1"
	"github.com/openkruise/kruise/pkg/util"
)

var testscheme *k8sruntime.Scheme

func init() {
	testscheme = k8sruntime.NewScheme()
	utilruntime.Must(v1.AddToScheme(testscheme))
	utilruntime.Must(appsv1alpha1.AddToScheme(testscheme))
}

func TestListAllCRRsRelatedWithNodePodProbe(t *testing.T) {
	testCase := []struct {
		name          string
		initObjs      []*appsv1alpha1.ContainerRecreateRequest
		nodePodProbe  appsv1alpha1.NodePodProbe
		expectCrrList *appsv1alpha1.ContainerRecreateRequestList
	}{
		{
			name: "case1: list all CRRs related with node pod probe",
			initObjs: []*appsv1alpha1.ContainerRecreateRequest{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fmt.Sprintf("%v-%v-%v", "test-nodepodprobe-1", flagEliveness, "1"),
						Namespace: "test-namespace-1",
						Labels: map[string]string{
							"owner": controllerName,
							"usage": "test-nodepodprobe-1",
						},
						ResourceVersion: "999",
					},
					Spec: appsv1alpha1.ContainerRecreateRequestSpec{
						PodName: "pod1",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fmt.Sprintf("%v-%v-%v", "test-nodepodprobe-1", flagEliveness, "2"),
						Namespace: "test-namespace-1",
						Labels: map[string]string{
							"owner": controllerName,
							"usage": "test-nodepodprobe-1",
						},
						ResourceVersion: "999",
					},
					Spec: appsv1alpha1.ContainerRecreateRequestSpec{
						PodName: "pod2",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fmt.Sprintf("%v-%v-%v", "test-nodepodprobe-1", flagEliveness, "3"),
						Namespace: "test-namespace-1",
						Labels: map[string]string{
							"owner": controllerName,
							"usage": "test-nodepodprobe-1",
						},
						ResourceVersion: "999",
					},
					Spec: appsv1alpha1.ContainerRecreateRequestSpec{
						PodName: "pod3",
					},
				},
			},
			nodePodProbe: appsv1alpha1.NodePodProbe{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-nodepodprobe-1",
				},
			},
			expectCrrList: &appsv1alpha1.ContainerRecreateRequestList{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ContainerRecreateRequestList",
					APIVersion: "apps.kruise.io/v1alpha1",
				},
				Items: []appsv1alpha1.ContainerRecreateRequest{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      fmt.Sprintf("%v-%v-%v", "test-nodepodprobe-1", flagEliveness, "1"),
							Namespace: "test-namespace-1",
							Labels: map[string]string{
								"owner": controllerName,
								"usage": "test-nodepodprobe-1",
							},
							ResourceVersion: "999",
						},
						Spec: appsv1alpha1.ContainerRecreateRequestSpec{
							PodName: "pod1",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      fmt.Sprintf("%v-%v-%v", "test-nodepodprobe-1", flagEliveness, "2"),
							Namespace: "test-namespace-1",
							Labels: map[string]string{
								"owner": controllerName,
								"usage": "test-nodepodprobe-1",
							},
							ResourceVersion: "999",
						},
						Spec: appsv1alpha1.ContainerRecreateRequestSpec{
							PodName: "pod2",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      fmt.Sprintf("%v-%v-%v", "test-nodepodprobe-1", flagEliveness, "3"),
							Namespace: "test-namespace-1",
							Labels: map[string]string{
								"owner": controllerName,
								"usage": "test-nodepodprobe-1",
							},
							ResourceVersion: "999",
						},
						Spec: appsv1alpha1.ContainerRecreateRequestSpec{
							PodName: "pod3",
						},
					},
				},
			},
		},
		{
			name: "case2: list all CRRs related with node pod probe(no found)",
			initObjs: []*appsv1alpha1.ContainerRecreateRequest{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fmt.Sprintf("%v-%v-%v", "test-nodepodprobe-1", flagEliveness, "1"),
						Namespace: "test-namespace-1",
						Labels: map[string]string{
							"owner": controllerName,
							"usage": "test-nodepodprobe-2",
						},
						ResourceVersion: "999",
					},
					Spec: appsv1alpha1.ContainerRecreateRequestSpec{
						PodName: "pod1",
					},
				},
			},
			nodePodProbe: appsv1alpha1.NodePodProbe{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-nodepodprobe-1",
				},
			},
			expectCrrList: &appsv1alpha1.ContainerRecreateRequestList{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ContainerRecreateRequestList",
					APIVersion: "apps.kruise.io/v1alpha1",
				},
				Items: []appsv1alpha1.ContainerRecreateRequest{},
			},
		},
	}

	for _, tc := range testCase {
		t.Run(tc.name, func(t *testing.T) {
			initClientObjs := []client.Object{}
			for i := range tc.initObjs {
				initClientObjs = append(initClientObjs, tc.initObjs[i])
			}
			handler := ReconcileEnhancedLivenessProbe{
				Client: fake.NewClientBuilder().WithScheme(testscheme).
					WithObjects(initClientObjs...).Build(),
			}
			got, err := handler.listAllCRRsRelatedWithNodePodProbe(context.TODO(), tc.nodePodProbe)
			if err != nil {
				t.Errorf("Failed to list all CRRs related with node pod probe, err: %v", err)
			}
			if !reflect.DeepEqual(got, tc.expectCrrList) {
				t.Errorf("No match, expect: %v, but: %v", util.DumpJSON(tc.expectCrrList), util.DumpJSON(got))
			}
		})
	}
}

func TestGetPodContainerProbeFromNodePodProbe(t *testing.T) {
	testCase := []struct {
		name                 string
		nodePodProbe         appsv1alpha1.NodePodProbe
		containerProbeName   string
		pod                  *v1.Pod
		expectContainerProbe appsv1alpha1.ContainerProbe
		expectBool           bool
	}{
		{
			name: "case1: exist pod container probe from nodePodProbe",
			nodePodProbe: appsv1alpha1.NodePodProbe{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-nodepodprobe-1",
				},
				Spec: appsv1alpha1.NodePodProbeSpec{
					PodProbes: []appsv1alpha1.PodProbe{
						{
							Name:      "pod1",
							Namespace: "pod1-namespace",
							UID:       "111-222-333",
							Probes: []appsv1alpha1.ContainerProbe{
								{
									Name:          "pod1-c1-" + flagEliveness,
									ContainerName: "c1",
									Probe: appsv1alpha1.ContainerProbeSpec{
										Probe: v1.Probe{
											ProbeHandler: v1.ProbeHandler{
												Exec: &v1.ExecAction{
													Command: []string{"/bin/sh", "-c", "/healthy.sh"},
												},
											},
										},
									},
								},
								{
									Name:          "pod1-c2-" + flagEliveness,
									ContainerName: "c2",
								},
							},
						},
						{
							Name:      "pod2",
							Namespace: "pod2-namespace",
							UID:       "123-123-123",
							Probes: []appsv1alpha1.ContainerProbe{
								{
									Name:          "pod1-c1-" + flagEliveness,
									ContainerName: "c1",
								},
								{
									Name:          "pod1-c2-" + flagEliveness,
									ContainerName: "c2",
									Probe: appsv1alpha1.ContainerProbeSpec{
										Probe: v1.Probe{
											ProbeHandler: v1.ProbeHandler{
												Exec: &v1.ExecAction{
													Command: []string{"/bin/sh", "-c", "/healthy.sh"},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			containerProbeName: "pod1-c1-" + flagEliveness,
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod1",
					Namespace: "pod1-namespace",
					UID:       "111-222-333",
				},
			},
			expectContainerProbe: appsv1alpha1.ContainerProbe{
				Name:          "pod1-c1-" + flagEliveness,
				ContainerName: "c1",
				Probe: appsv1alpha1.ContainerProbeSpec{
					Probe: v1.Probe{
						ProbeHandler: v1.ProbeHandler{
							Exec: &v1.ExecAction{
								Command: []string{"/bin/sh", "-c", "/healthy.sh"},
							},
						},
					},
				},
			},
			expectBool: true,
		},
		{
			name: "case2: no found pod container probe from nodePodProbe",
			nodePodProbe: appsv1alpha1.NodePodProbe{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-nodepodprobe-1",
				},
				Spec: appsv1alpha1.NodePodProbeSpec{
					PodProbes: []appsv1alpha1.PodProbe{
						{
							Name:      "pod1",
							Namespace: "pod1-namespace",
							UID:       "111-222-333",
							Probes: []appsv1alpha1.ContainerProbe{
								{
									Name:          "pod1-c1-" + flagEliveness,
									ContainerName: "c1",
									Probe: appsv1alpha1.ContainerProbeSpec{
										Probe: v1.Probe{
											ProbeHandler: v1.ProbeHandler{
												Exec: &v1.ExecAction{
													Command: []string{"/bin/sh", "-c", "/healthy.sh"},
												},
											},
										},
									},
								},
								{
									Name:          "pod1-c2-" + flagEliveness,
									ContainerName: "c2",
								},
							},
						},
						{
							Name:      "pod2",
							Namespace: "pod2-namespace",
							UID:       "123-123-123",
							Probes: []appsv1alpha1.ContainerProbe{
								{
									Name:          "pod1-c1-" + flagEliveness,
									ContainerName: "c1",
								},
								{
									Name:          "pod1-c2-" + flagEliveness,
									ContainerName: "c2",
									Probe: appsv1alpha1.ContainerProbeSpec{
										Probe: v1.Probe{
											ProbeHandler: v1.ProbeHandler{
												Exec: &v1.ExecAction{
													Command: []string{"/bin/sh", "-c", "/healthy.sh"},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			containerProbeName: "pod3-c1-" + flagEliveness,
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod3",
					Namespace: "pod3-namespace",
					UID:       "111-222-333",
				},
			},
			expectContainerProbe: appsv1alpha1.ContainerProbe{},
			expectBool:           false,
		},
	}
	for _, tc := range testCase {
		t.Run(tc.name, func(t *testing.T) {
			got, found := getPodContainerProbeFromNodePodProbe(tc.nodePodProbe, tc.containerProbeName, tc.pod)
			if found != tc.expectBool {
				t.Errorf("No match expect: %v, but: %v", tc.expectBool, found)
			}
			if !reflect.DeepEqual(tc.expectContainerProbe, got) {
				t.Errorf("No match, expect: %v, but: %v", util.DumpJSON(tc.expectContainerProbe), util.DumpJSON(got))
			}
		})
	}
}

func TestGetFailedLivenessProbeFromNodePodProbe(t *testing.T) {
	testCase := []struct {
		name         string
		nodePodProbe appsv1alpha1.NodePodProbe
		initPods     []*v1.Pod
		expect       []podLivenessFailedContainers
	}{
		{
			name: "case1: found failed liveness probe from nodePodProbe",
			nodePodProbe: appsv1alpha1.NodePodProbe{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-nodepodprobe-1",
				},
				Spec: appsv1alpha1.NodePodProbeSpec{
					PodProbes: []appsv1alpha1.PodProbe{
						{
							Name:      "pod1",
							Namespace: "pod1-namespace",
							UID:       "111-222-333",
							Probes: []appsv1alpha1.ContainerProbe{
								{
									Name:          fmt.Sprintf("pod1-c1-%v", flagEliveness),
									ContainerName: "c1",
									Probe: appsv1alpha1.ContainerProbeSpec{
										Probe: v1.Probe{
											ProbeHandler: v1.ProbeHandler{
												Exec: &v1.ExecAction{
													Command: []string{"/bin/sh", "-c", "/healthy.sh"},
												},
											},
										},
									},
								},
								{
									Name:          "pod1-c2",
									ContainerName: "c1",
									Probe: appsv1alpha1.ContainerProbeSpec{
										Probe: v1.Probe{
											ProbeHandler: v1.ProbeHandler{
												HTTPGet: &v1.HTTPGetAction{
													Port: intstr.IntOrString{
														Type:   intstr.Int,
														IntVal: 9090,
													},
												},
											},
										},
									},
								},
							},
						},
						{
							Name:      "pod2",
							Namespace: "pod2-namespace",
							UID:       "123-222-333",
							Probes: []appsv1alpha1.ContainerProbe{
								{
									Name:          fmt.Sprintf("pod2-c1-%v", flagEliveness),
									ContainerName: "c1",
									Probe: appsv1alpha1.ContainerProbeSpec{
										Probe: v1.Probe{
											ProbeHandler: v1.ProbeHandler{
												Exec: &v1.ExecAction{
													Command: []string{"/bin/sh", "-c", "/healthy.sh"},
												},
											},
										},
									},
								},
								{
									Name:          fmt.Sprintf("pod2-c2-%v", flagEliveness),
									ContainerName: "c2",
									Probe: appsv1alpha1.ContainerProbeSpec{
										Probe: v1.Probe{
											ProbeHandler: v1.ProbeHandler{
												Exec: &v1.ExecAction{
													Command: []string{"/bin/sh", "-c", "/healthy.sh"},
												},
											},
										},
									},
								},
							},
						},
					},
				},
				Status: appsv1alpha1.NodePodProbeStatus{
					PodProbeStatuses: []appsv1alpha1.PodProbeStatus{
						{
							Name:      "pod1",
							Namespace: "pod1-namespace",
							UID:       "111-222-333",
							ProbeStates: []appsv1alpha1.ContainerProbeState{
								{
									Name:  fmt.Sprintf("pod1-c1-%v", flagEliveness),
									State: appsv1alpha1.ProbeFailed,
								},
								{
									Name:  "pod1-c2",
									State: appsv1alpha1.ProbeSucceeded,
								},
							},
						},
						{
							Name:      "pod2",
							Namespace: "pod2-namespace",
							UID:       "123-222-333",
							ProbeStates: []appsv1alpha1.ContainerProbeState{
								{
									Name:  fmt.Sprintf("pod2-c1-%v", flagEliveness),
									State: appsv1alpha1.ProbeSucceeded,
								},
								{
									Name:  fmt.Sprintf("pod2-c2-%v", flagEliveness),
									State: appsv1alpha1.ProbeSucceeded,
								},
							},
						},
					},
				},
			},
			initPods: []*v1.Pod{
				{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Pod",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "pod1",
						Namespace:       "pod1-namespace",
						UID:             "111-222-333",
						ResourceVersion: "999",
						Annotations: map[string]string{
							alpha1.AnnotationUsingEnhancedLiveness: "true",
						},
					},
				},
				{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Pod",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "pod2",
						Namespace:       "pod2-namespace",
						UID:             "123-222-333",
						ResourceVersion: "999",
					},
				},
			},
			expect: []podLivenessFailedContainers{
				{
					Pod: &v1.Pod{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Pod",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:            "pod1",
							Namespace:       "pod1-namespace",
							UID:             "111-222-333",
							ResourceVersion: "999",
							Annotations: map[string]string{
								alpha1.AnnotationUsingEnhancedLiveness: "true",
							},
						},
					},
					FailedContainers: []string{
						"c1",
					},
				},
			},
		},
		{
			name: "case2: no found failed liveness probe from nodePodProbe",
			nodePodProbe: appsv1alpha1.NodePodProbe{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-nodepodprobe-1",
				},
				Spec: appsv1alpha1.NodePodProbeSpec{
					PodProbes: []appsv1alpha1.PodProbe{
						{
							Name:      "pod1",
							Namespace: "pod1-namespace",
							UID:       "111-222-333",
							Probes: []appsv1alpha1.ContainerProbe{
								{
									Name:          fmt.Sprintf("pod1-c1-%v", flagEliveness),
									ContainerName: "c1",
									Probe: appsv1alpha1.ContainerProbeSpec{
										Probe: v1.Probe{
											ProbeHandler: v1.ProbeHandler{
												Exec: &v1.ExecAction{
													Command: []string{"/bin/sh", "-c", "/healthy.sh"},
												},
											},
										},
									},
								},
								{
									Name:          "pod1-c2",
									ContainerName: "c1",
									Probe: appsv1alpha1.ContainerProbeSpec{
										Probe: v1.Probe{
											ProbeHandler: v1.ProbeHandler{
												HTTPGet: &v1.HTTPGetAction{
													Port: intstr.IntOrString{
														Type:   intstr.Int,
														IntVal: 9090,
													},
												},
											},
										},
									},
								},
							},
						},
						{
							Name:      "pod2",
							Namespace: "pod2-namespace",
							UID:       "123-222-333",
							Probes: []appsv1alpha1.ContainerProbe{
								{
									Name:          fmt.Sprintf("pod2-c1-%v", flagEliveness),
									ContainerName: "c1",
									Probe: appsv1alpha1.ContainerProbeSpec{
										Probe: v1.Probe{
											ProbeHandler: v1.ProbeHandler{
												Exec: &v1.ExecAction{
													Command: []string{"/bin/sh", "-c", "/healthy.sh"},
												},
											},
										},
									},
								},
								{
									Name:          fmt.Sprintf("pod2-c2-%v", flagEliveness),
									ContainerName: "c2",
									Probe: appsv1alpha1.ContainerProbeSpec{
										Probe: v1.Probe{
											ProbeHandler: v1.ProbeHandler{
												Exec: &v1.ExecAction{
													Command: []string{"/bin/sh", "-c", "/healthy.sh"},
												},
											},
										},
									},
								},
							},
						},
					},
				},
				Status: appsv1alpha1.NodePodProbeStatus{
					PodProbeStatuses: []appsv1alpha1.PodProbeStatus{
						{
							Name:      "pod1",
							Namespace: "pod1-namespace",
							UID:       "111-222-333",
							ProbeStates: []appsv1alpha1.ContainerProbeState{
								{
									Name:  fmt.Sprintf("pod1-c1-%v", flagEliveness),
									State: appsv1alpha1.ProbeFailed,
								},
								{
									Name:  "pod1-c2",
									State: appsv1alpha1.ProbeSucceeded,
								},
							},
						},
						{
							Name:      "pod2",
							Namespace: "pod2-namespace",
							UID:       "123-222-333",
							ProbeStates: []appsv1alpha1.ContainerProbeState{
								{
									Name:  fmt.Sprintf("pod2-c1-%v", flagEliveness),
									State: appsv1alpha1.ProbeSucceeded,
								},
								{
									Name:  fmt.Sprintf("pod2-c2-%v", flagEliveness),
									State: appsv1alpha1.ProbeSucceeded,
								},
							},
						},
					},
				},
			},
			initPods: []*v1.Pod{
				{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Pod",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "pod1",
						Namespace:       "pod1-namespace",
						UID:             "111-222-333",
						ResourceVersion: "999",
					},
				},
				{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Pod",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "pod2",
						Namespace:       "pod2-namespace",
						UID:             "123-222-333",
						ResourceVersion: "999",
					},
				},
			},
			expect: []podLivenessFailedContainers{},
		},
	}

	for _, tc := range testCase {
		t.Run(tc.name, func(t *testing.T) {
			initClientObjs := []client.Object{}
			for i := range tc.initPods {
				initClientObjs = append(initClientObjs, tc.initPods[i])
			}
			initClientObjs = append(initClientObjs, &tc.nodePodProbe)
			handler := ReconcileEnhancedLivenessProbe{
				Client: fake.NewClientBuilder().WithScheme(testscheme).
					WithObjects(initClientObjs...).Build(),
			}
			got, err := handler.getFailedLivenessProbeFromNodePodProbe(tc.nodePodProbe)
			if err != nil {
				t.Errorf("Failed to get failed liveness probe from nodePodPrbe, err: %v", err)
			}
			if !reflect.DeepEqual(tc.expect, got) {
				t.Logf("No match, expect: %v, got: %v", util.DumpJSON(tc.expect), util.DumpJSON(got))
			}
		})
	}
}

func TestSyncNodePodProbe(t *testing.T) {
	isController := true
	ttLSecondsAfterFinished := int32(600)
	activeDeadlineSeconds := int64(1800) // 30min = 60s * 30 = 1800s
	utTest = true

	testCase := []struct {
		name         string
		nodePodProbe appsv1alpha1.NodePodProbe
		initPods     []*v1.Pod
		crrs         []appsv1alpha1.ContainerRecreateRequest
		expectCRRs   *appsv1alpha1.ContainerRecreateRequestList
	}{
		{
			name: "case1: generate a new CRR for nodePodProbe",
			// there are four pods allocated to this node that are related with nodePodProbe object
			// there are one crr related with nodePodProbe
			nodePodProbe: appsv1alpha1.NodePodProbe{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-nodepodprobe-1",
				},
				Spec: appsv1alpha1.NodePodProbeSpec{
					PodProbes: []appsv1alpha1.PodProbe{
						{
							Name:      "pod1",
							Namespace: "pod1-namespace",
							UID:       "111-222-333",
							Probes: []appsv1alpha1.ContainerProbe{
								{
									Name:          fmt.Sprintf("pod1-c1-%v", flagEliveness),
									ContainerName: "c1",
									Probe: appsv1alpha1.ContainerProbeSpec{
										Probe: v1.Probe{
											ProbeHandler: v1.ProbeHandler{
												Exec: &v1.ExecAction{
													Command: []string{"/bin/sh", "-c", "/healthy.sh"},
												},
											},
										},
									},
								},
								{
									Name:          fmt.Sprintf("pod1-c2-%v", flagEliveness),
									ContainerName: "c2",
									Probe: appsv1alpha1.ContainerProbeSpec{
										Probe: v1.Probe{
											ProbeHandler: v1.ProbeHandler{
												HTTPGet: &v1.HTTPGetAction{
													Port: intstr.IntOrString{
														Type:   intstr.Int,
														IntVal: 9090,
													},
												},
											},
										},
									},
								},
							},
						},
						{
							Name:      "pod2",
							Namespace: "pod2-namespace",
							UID:       "222-222-333",
							Probes: []appsv1alpha1.ContainerProbe{
								{
									Name:          fmt.Sprintf("pod2-c1-%v", flagEliveness),
									ContainerName: "c1",
									Probe: appsv1alpha1.ContainerProbeSpec{
										Probe: v1.Probe{
											ProbeHandler: v1.ProbeHandler{
												Exec: &v1.ExecAction{
													Command: []string{"/bin/sh", "-c", "/healthy.sh"},
												},
											},
										},
									},
								},
								{
									Name:          fmt.Sprintf("pod2-c2-%v", flagEliveness),
									ContainerName: "c2",
									Probe: appsv1alpha1.ContainerProbeSpec{
										Probe: v1.Probe{
											ProbeHandler: v1.ProbeHandler{
												Exec: &v1.ExecAction{
													Command: []string{"/bin/sh", "-c", "/healthy.sh"},
												},
											},
										},
									},
								},
							},
						},
						{
							Name:      "pod3",
							Namespace: "pod3-namespace",
							UID:       "333-222-333",
							Probes: []appsv1alpha1.ContainerProbe{
								{
									Name:          fmt.Sprintf("pod3-c1-%v", flagEliveness),
									ContainerName: "c1",
									Probe: appsv1alpha1.ContainerProbeSpec{
										Probe: v1.Probe{
											ProbeHandler: v1.ProbeHandler{
												Exec: &v1.ExecAction{
													Command: []string{"/bin/sh", "-c", "/healthy.sh"},
												},
											},
										},
									},
								},
								{
									Name:          "pod3-c2",
									ContainerName: "c2",
									Probe: appsv1alpha1.ContainerProbeSpec{
										Probe: v1.Probe{
											ProbeHandler: v1.ProbeHandler{
												HTTPGet: &v1.HTTPGetAction{
													Path:   "/index.html",
													Port:   intstr.IntOrString{Type: intstr.Int, StrVal: "9090"},
													Scheme: v1.URISchemeHTTP,
												},
											},
										},
									},
								},
							},
						},
						{
							Name:      "pod4",
							Namespace: "pod4-namespace",
							UID:       "444-222-333",
							Probes: []appsv1alpha1.ContainerProbe{
								{
									Name:          "pod4-c1",
									ContainerName: "c1",
									Probe: appsv1alpha1.ContainerProbeSpec{
										Probe: v1.Probe{
											ProbeHandler: v1.ProbeHandler{
												HTTPGet: &v1.HTTPGetAction{
													Path:   "/index.html",
													Port:   intstr.IntOrString{Type: intstr.Int, StrVal: "9090"},
													Scheme: v1.URISchemeHTTP,
												},
											},
										},
									},
								},
								{
									Name:          "pod4-c2",
									ContainerName: "c2",
									Probe: appsv1alpha1.ContainerProbeSpec{
										Probe: v1.Probe{
											ProbeHandler: v1.ProbeHandler{
												HTTPGet: &v1.HTTPGetAction{
													Path:   "/index.html",
													Port:   intstr.IntOrString{Type: intstr.Int, StrVal: "9090"},
													Scheme: v1.URISchemeHTTP,
												},
											},
										},
									},
								},
							},
						},
					},
				},
				Status: appsv1alpha1.NodePodProbeStatus{
					PodProbeStatuses: []appsv1alpha1.PodProbeStatus{
						{
							Name:      "pod1",
							Namespace: "pod1-namespace",
							UID:       "111-222-333",
							// exist a failed probe check result for pod1-c1
							ProbeStates: []appsv1alpha1.ContainerProbeState{
								{
									Name:  fmt.Sprintf("pod1-c1-%v", flagEliveness),
									State: appsv1alpha1.ProbeFailed,
								},
								{
									Name:  fmt.Sprintf("pod1-c2-%v", flagEliveness),
									State: appsv1alpha1.ProbeSucceeded,
								},
							},
						},
						{
							Name:      "pod2",
							Namespace: "pod2-namespace",
							UID:       "222-222-333",
							ProbeStates: []appsv1alpha1.ContainerProbeState{
								{
									Name:  fmt.Sprintf("pod2-c1-%v", flagEliveness),
									State: appsv1alpha1.ProbeSucceeded,
								},
								{
									Name:  fmt.Sprintf("pod2-c2-%v", flagEliveness),
									State: appsv1alpha1.ProbeSucceeded,
								},
							},
						},
						{
							Name:      "pod3",
							Namespace: "pod3-namespace",
							UID:       "333-222-333",
							ProbeStates: []appsv1alpha1.ContainerProbeState{
								{
									Name:  fmt.Sprintf("pod3-c1-%v", flagEliveness),
									State: appsv1alpha1.ProbeFailed,
								},
								{
									Name:  "pod3-c2",
									State: appsv1alpha1.ProbeSucceeded,
								},
							},
						},
						{
							Name:      "pod4",
							Namespace: "pod4-namespace",
							UID:       "444-222-333",
							ProbeStates: []appsv1alpha1.ContainerProbeState{
								{
									Name:  "pod4-c1",
									State: appsv1alpha1.ProbeSucceeded,
								},
								{
									Name:  "pod4-c2",
									State: appsv1alpha1.ProbeSucceeded,
								},
							},
						},
					},
				},
			},
			crrs: []appsv1alpha1.ContainerRecreateRequest{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "pod3-namespace",
						Name:      fmt.Sprintf("test-nodepodprobe-1-%v-idwexd", flagEliveness),
						Labels: map[string]string{
							"owner":   controllerName,
							"usage":   "test-nodepodprobe-1",
							podUIDKey: "333-222-333",
						},
						ResourceVersion: "999",
						OwnerReferences: []metav1.OwnerReference{
							{
								Name:       "test-nodepodprobe-1",
								Kind:       "NodePodProbe",
								Controller: &isController,
							},
						},
					},
					Spec: appsv1alpha1.ContainerRecreateRequestSpec{
						PodName: "pod3",
						Containers: []appsv1alpha1.ContainerRecreateRequestContainer{
							{
								Name: "c1",
							},
						},
						Strategy: &appsv1alpha1.ContainerRecreateRequestStrategy{
							FailurePolicy: appsv1alpha1.ContainerRecreateRequestFailurePolicyFail,
							ForceRecreate: true,
						},
						ActiveDeadlineSeconds:   &activeDeadlineSeconds,
						TTLSecondsAfterFinished: &ttLSecondsAfterFinished,
					},
					Status: appsv1alpha1.ContainerRecreateRequestStatus{
						Phase: appsv1alpha1.ContainerRecreateRequestRecreating,
					},
				},
			},
			initPods: []*v1.Pod{
				{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Pod",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "pod1",
						Namespace:       "pod1-namespace",
						UID:             "111-222-333",
						ResourceVersion: "999",
						Annotations: map[string]string{
							alpha1.AnnotationUsingEnhancedLiveness: "true",
						},
					},
				},
				{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Pod",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "pod2",
						Namespace:       "pod2-namespace",
						UID:             "123-222-333",
						ResourceVersion: "999",
						Annotations: map[string]string{
							alpha1.AnnotationUsingEnhancedLiveness: "true",
						},
					},
				},
				{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Pod",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "pod3",
						Namespace:       "pod3-namespace",
						UID:             "333-222-333",
						ResourceVersion: "999",
						Annotations: map[string]string{
							alpha1.AnnotationUsingEnhancedLiveness: "true",
						},
					},
				},
				{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Pod",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "pod4",
						Namespace:       "pod4-namespace",
						UID:             "444-222-333",
						ResourceVersion: "999",
						Annotations: map[string]string{
							alpha1.AnnotationUsingEnhancedLiveness: "true",
						},
					},
				},
			},
			expectCRRs: &appsv1alpha1.ContainerRecreateRequestList{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ContainerRecreateRequestList",
					APIVersion: "apps.kruise.io/v1alpha1",
				},
				Items: []appsv1alpha1.ContainerRecreateRequest{
					{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: "pod1-namespace",
							Labels: map[string]string{
								"owner":   controllerName,
								"usage":   "test-nodepodprobe-1",
								podUIDKey: "111-222-333",
							},
							OwnerReferences: []metav1.OwnerReference{
								{
									Name:       "test-nodepodprobe-1",
									Kind:       "NodePodProbe",
									Controller: &isController,
								},
							},
						},
						Spec: appsv1alpha1.ContainerRecreateRequestSpec{
							PodName: "pod1",
							Containers: []appsv1alpha1.ContainerRecreateRequestContainer{
								{
									Name: "c1",
								},
							},
							Strategy: &appsv1alpha1.ContainerRecreateRequestStrategy{
								FailurePolicy: appsv1alpha1.ContainerRecreateRequestFailurePolicyFail,
								ForceRecreate: true,
							},
							ActiveDeadlineSeconds:   &activeDeadlineSeconds,
							TTLSecondsAfterFinished: &ttLSecondsAfterFinished,
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: "pod3-namespace",
							Labels: map[string]string{
								"owner":   controllerName,
								"usage":   "test-nodepodprobe-1",
								podUIDKey: "333-222-333",
							},
							OwnerReferences: []metav1.OwnerReference{
								{
									Name:       "test-nodepodprobe-1",
									Kind:       "NodePodProbe",
									Controller: &isController,
								},
							},
						},
						Spec: appsv1alpha1.ContainerRecreateRequestSpec{
							PodName: "pod3",
							Containers: []appsv1alpha1.ContainerRecreateRequestContainer{
								{
									Name: "c1",
								},
							},
							Strategy: &appsv1alpha1.ContainerRecreateRequestStrategy{
								FailurePolicy: appsv1alpha1.ContainerRecreateRequestFailurePolicyFail,
								ForceRecreate: true,
							},
							ActiveDeadlineSeconds:   &activeDeadlineSeconds,
							TTLSecondsAfterFinished: &ttLSecondsAfterFinished,
						},
						Status: appsv1alpha1.ContainerRecreateRequestStatus{
							Phase: appsv1alpha1.ContainerRecreateRequestRecreating,
						},
					},
				},
			},
		},
		{
			name: "case2: no found config in nodePodProbe that related with some CRRs",
			// there are some CRRs related with nodePodProbe, but no config in nodePodProbe.
			// these CRRs will be removed.
			nodePodProbe: appsv1alpha1.NodePodProbe{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-nodepodprobe-1",
				},
				Spec: appsv1alpha1.NodePodProbeSpec{
					PodProbes: []appsv1alpha1.PodProbe{
						{
							Name:      "pod1",
							Namespace: "pod1-namespace",
							UID:       "111-222-333",
							Probes: []appsv1alpha1.ContainerProbe{
								{
									Name:          fmt.Sprintf("pod1-c1-%v", flagEliveness),
									ContainerName: "c1",
									Probe: appsv1alpha1.ContainerProbeSpec{
										Probe: v1.Probe{
											ProbeHandler: v1.ProbeHandler{
												Exec: &v1.ExecAction{
													Command: []string{"/bin/sh", "-c", "/healthy.sh"},
												},
											},
										},
									},
								},
								{
									Name:          fmt.Sprintf("pod1-c2-%v", flagEliveness),
									ContainerName: "c2",
									Probe: appsv1alpha1.ContainerProbeSpec{
										Probe: v1.Probe{
											ProbeHandler: v1.ProbeHandler{
												HTTPGet: &v1.HTTPGetAction{
													Port: intstr.IntOrString{
														Type:   intstr.Int,
														IntVal: 9090,
													},
												},
											},
										},
									},
								},
							},
						},
						{
							Name:      "pod2",
							Namespace: "pod2-namespace",
							UID:       "222-222-333",
							Probes: []appsv1alpha1.ContainerProbe{
								{
									Name:          fmt.Sprintf("pod2-c1-%v", flagEliveness),
									ContainerName: "c1",
									Probe: appsv1alpha1.ContainerProbeSpec{
										Probe: v1.Probe{
											ProbeHandler: v1.ProbeHandler{
												Exec: &v1.ExecAction{
													Command: []string{"/bin/sh", "-c", "/healthy.sh"},
												},
											},
										},
									},
								},
								{
									Name:          fmt.Sprintf("pod2-c2-%v", flagEliveness),
									ContainerName: "c2",
									Probe: appsv1alpha1.ContainerProbeSpec{
										Probe: v1.Probe{
											ProbeHandler: v1.ProbeHandler{
												Exec: &v1.ExecAction{
													Command: []string{"/bin/sh", "-c", "/healthy.sh"},
												},
											},
										},
									},
								},
							},
						},
						{
							Name:      "pod3",
							Namespace: "pod3-namespace",
							UID:       "333-222-333",
							Probes: []appsv1alpha1.ContainerProbe{
								{
									Name:          fmt.Sprintf("pod3-c1-%v", flagEliveness),
									ContainerName: "c1",
									Probe: appsv1alpha1.ContainerProbeSpec{
										Probe: v1.Probe{
											ProbeHandler: v1.ProbeHandler{
												Exec: &v1.ExecAction{
													Command: []string{"/bin/sh", "-c", "/healthy.sh"},
												},
											},
										},
									},
								},
								{
									Name:          "pod3-c2",
									ContainerName: "c2",
									Probe: appsv1alpha1.ContainerProbeSpec{
										Probe: v1.Probe{
											ProbeHandler: v1.ProbeHandler{
												HTTPGet: &v1.HTTPGetAction{
													Path:   "/index.html",
													Port:   intstr.IntOrString{Type: intstr.Int, StrVal: "9090"},
													Scheme: v1.URISchemeHTTP,
												},
											},
										},
									},
								},
							},
						},
						{
							Name:      "pod4",
							Namespace: "pod4-namespace",
							UID:       "444-222-333",
							Probes: []appsv1alpha1.ContainerProbe{
								{
									Name:          "pod4-c1",
									ContainerName: "c1",
									Probe: appsv1alpha1.ContainerProbeSpec{
										Probe: v1.Probe{
											ProbeHandler: v1.ProbeHandler{
												HTTPGet: &v1.HTTPGetAction{
													Path:   "/index.html",
													Port:   intstr.IntOrString{Type: intstr.Int, StrVal: "9090"},
													Scheme: v1.URISchemeHTTP,
												},
											},
										},
									},
								},
								{
									Name:          "pod4-c2",
									ContainerName: "c2",
									Probe: appsv1alpha1.ContainerProbeSpec{
										Probe: v1.Probe{
											ProbeHandler: v1.ProbeHandler{
												HTTPGet: &v1.HTTPGetAction{
													Path:   "/index.html",
													Port:   intstr.IntOrString{Type: intstr.Int, StrVal: "9090"},
													Scheme: v1.URISchemeHTTP,
												},
											},
										},
									},
								},
							},
						},
					},
				},
				Status: appsv1alpha1.NodePodProbeStatus{
					PodProbeStatuses: []appsv1alpha1.PodProbeStatus{
						{
							Name:      "pod1",
							Namespace: "pod1-namespace",
							UID:       "111-222-333",
							// exist a failed probe check result for pod1-c1
							ProbeStates: []appsv1alpha1.ContainerProbeState{
								{
									Name:  fmt.Sprintf("pod1-c1-%v", flagEliveness),
									State: appsv1alpha1.ProbeFailed,
								},
								{
									Name:  fmt.Sprintf("pod1-c2-%v", flagEliveness),
									State: appsv1alpha1.ProbeSucceeded,
								},
							},
						},
						{
							Name:      "pod2",
							Namespace: "pod2-namespace",
							UID:       "222-222-333",
							ProbeStates: []appsv1alpha1.ContainerProbeState{
								{
									Name:  fmt.Sprintf("pod2-c1-%v", flagEliveness),
									State: appsv1alpha1.ProbeFailed,
								},
								{
									Name:  fmt.Sprintf("pod2-c2-%v", flagEliveness),
									State: appsv1alpha1.ProbeSucceeded,
								},
							},
						},
						{
							Name:      "pod3",
							Namespace: "pod3-namespace",
							UID:       "333-222-333",
							ProbeStates: []appsv1alpha1.ContainerProbeState{
								{
									Name:  fmt.Sprintf("pod3-c1-%v", flagEliveness),
									State: appsv1alpha1.ProbeFailed,
								},
								{
									Name:  "pod3-c2",
									State: appsv1alpha1.ProbeSucceeded,
								},
							},
						},
						{
							Name:      "pod4",
							Namespace: "pod4-namespace",
							UID:       "444-222-333",
							ProbeStates: []appsv1alpha1.ContainerProbeState{
								{
									Name:  "pod4-c1",
									State: appsv1alpha1.ProbeSucceeded,
								},
								{
									Name:  "pod4-c2",
									State: appsv1alpha1.ProbeSucceeded,
								},
							},
						},
					},
				},
			},
			crrs: []appsv1alpha1.ContainerRecreateRequest{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "pod3-namespace",
						Name:      fmt.Sprintf("test-nodepodprobe-1-%v-idwexd", flagEliveness),
						Labels: map[string]string{
							"owner": controllerName,
							"usage": "test-nodepodprobe-1",
							// no found probe config in this nodePodProbe
							podUIDKey: "333-222-999",
						},
						ResourceVersion: "999",
						OwnerReferences: []metav1.OwnerReference{
							{
								Name:       "test-nodepodprobe-1",
								Kind:       "NodePodProbe",
								Controller: &isController,
							},
						},
					},
					Spec: appsv1alpha1.ContainerRecreateRequestSpec{
						PodName: "pod3",
						Containers: []appsv1alpha1.ContainerRecreateRequestContainer{
							{
								Name: "c1",
							},
						},
						Strategy: &appsv1alpha1.ContainerRecreateRequestStrategy{
							FailurePolicy: appsv1alpha1.ContainerRecreateRequestFailurePolicyFail,
							ForceRecreate: true,
						},
						ActiveDeadlineSeconds:   &activeDeadlineSeconds,
						TTLSecondsAfterFinished: &ttLSecondsAfterFinished,
					},
					Status: appsv1alpha1.ContainerRecreateRequestStatus{
						Phase: appsv1alpha1.ContainerRecreateRequestRecreating,
					},
				},
			},
			initPods: []*v1.Pod{
				{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Pod",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "pod1",
						Namespace:       "pod1-namespace",
						UID:             "111-222-333",
						ResourceVersion: "999",
						Annotations: map[string]string{
							alpha1.AnnotationUsingEnhancedLiveness: "true",
						},
					},
				},
				{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Pod",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "pod2",
						Namespace:       "pod2-namespace",
						UID:             "222-222-333",
						ResourceVersion: "999",
						Annotations: map[string]string{
							alpha1.AnnotationUsingEnhancedLiveness: "true",
						},
					},
				},
				{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Pod",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "pod3",
						Namespace:       "pod3-namespace",
						UID:             "333-222-333",
						ResourceVersion: "999",
						Annotations: map[string]string{
							alpha1.AnnotationUsingEnhancedLiveness: "true",
						},
					},
				},
				{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Pod",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "pod4",
						Namespace:       "pod4-namespace",
						UID:             "444-222-333",
						ResourceVersion: "999",
						Annotations: map[string]string{
							alpha1.AnnotationUsingEnhancedLiveness: "true",
						},
					},
				},
			},
			expectCRRs: &appsv1alpha1.ContainerRecreateRequestList{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ContainerRecreateRequestList",
					APIVersion: "apps.kruise.io/v1alpha1",
				},
				Items: []appsv1alpha1.ContainerRecreateRequest{
					{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: "pod1-namespace",
							Labels: map[string]string{
								"owner":   controllerName,
								"usage":   "test-nodepodprobe-1",
								podUIDKey: "111-222-333",
							},
							OwnerReferences: []metav1.OwnerReference{
								{
									Name:       "test-nodepodprobe-1",
									Kind:       "NodePodProbe",
									Controller: &isController,
								},
							},
						},
						Spec: appsv1alpha1.ContainerRecreateRequestSpec{
							PodName: "pod1",
							Containers: []appsv1alpha1.ContainerRecreateRequestContainer{
								{
									Name: "c1",
								},
							},
							Strategy: &appsv1alpha1.ContainerRecreateRequestStrategy{
								FailurePolicy: appsv1alpha1.ContainerRecreateRequestFailurePolicyFail,
								ForceRecreate: true,
							},
							ActiveDeadlineSeconds:   &activeDeadlineSeconds,
							TTLSecondsAfterFinished: &ttLSecondsAfterFinished,
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: "pod2-namespace",
							Labels: map[string]string{
								"owner":   controllerName,
								"usage":   "test-nodepodprobe-1",
								podUIDKey: "222-222-333",
							},
							OwnerReferences: []metav1.OwnerReference{
								{
									Name:       "test-nodepodprobe-1",
									Kind:       "NodePodProbe",
									Controller: &isController,
								},
							},
						},
						Spec: appsv1alpha1.ContainerRecreateRequestSpec{
							PodName: "pod2",
							Containers: []appsv1alpha1.ContainerRecreateRequestContainer{
								{
									Name: "c1",
								},
							},
							Strategy: &appsv1alpha1.ContainerRecreateRequestStrategy{
								FailurePolicy: appsv1alpha1.ContainerRecreateRequestFailurePolicyFail,
								ForceRecreate: true,
							},
							ActiveDeadlineSeconds:   &activeDeadlineSeconds,
							TTLSecondsAfterFinished: &ttLSecondsAfterFinished,
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: "pod3-namespace",
							Labels: map[string]string{
								"owner":   controllerName,
								"usage":   "test-nodepodprobe-1",
								podUIDKey: "333-222-333",
							},
							OwnerReferences: []metav1.OwnerReference{
								{
									Name:       "test-nodepodprobe-1",
									Kind:       "NodePodProbe",
									Controller: &isController,
								},
							},
						},
						Spec: appsv1alpha1.ContainerRecreateRequestSpec{
							PodName: "pod3",
							Containers: []appsv1alpha1.ContainerRecreateRequestContainer{
								{
									Name: "c1",
								},
							},
							Strategy: &appsv1alpha1.ContainerRecreateRequestStrategy{
								FailurePolicy: appsv1alpha1.ContainerRecreateRequestFailurePolicyFail,
								ForceRecreate: true,
							},
							ActiveDeadlineSeconds:   &activeDeadlineSeconds,
							TTLSecondsAfterFinished: &ttLSecondsAfterFinished,
						},
					},
				},
			},
		},
		{
			// for this case, all CRRs related with this nodePodProbe have been created.
			// So the controller don't need to create new CRRs.
			name: "case3: already exist CRRs related with this nodePodProbe",
			nodePodProbe: appsv1alpha1.NodePodProbe{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-nodepodprobe-1",
				},
				Spec: appsv1alpha1.NodePodProbeSpec{
					PodProbes: []appsv1alpha1.PodProbe{
						{
							Name:      "pod1",
							Namespace: "pod1-namespace",
							UID:       "111-222-333",
							Probes: []appsv1alpha1.ContainerProbe{
								{
									Name:          fmt.Sprintf("pod1-c1-%v", flagEliveness),
									ContainerName: "c1",
									Probe: appsv1alpha1.ContainerProbeSpec{
										Probe: v1.Probe{
											ProbeHandler: v1.ProbeHandler{
												Exec: &v1.ExecAction{
													Command: []string{"/bin/sh", "-c", "/healthy.sh"},
												},
											},
										},
									},
								},
								{
									Name:          fmt.Sprintf("pod1-c2-%v", flagEliveness),
									ContainerName: "c2",
									Probe: appsv1alpha1.ContainerProbeSpec{
										Probe: v1.Probe{
											ProbeHandler: v1.ProbeHandler{
												HTTPGet: &v1.HTTPGetAction{
													Port: intstr.IntOrString{
														Type:   intstr.Int,
														IntVal: 9090,
													},
												},
											},
										},
									},
								},
							},
						},
						{
							Name:      "pod2",
							Namespace: "pod2-namespace",
							UID:       "222-222-333",
							Probes: []appsv1alpha1.ContainerProbe{
								{
									Name:          fmt.Sprintf("pod2-c1-%v", flagEliveness),
									ContainerName: "c1",
									Probe: appsv1alpha1.ContainerProbeSpec{
										Probe: v1.Probe{
											ProbeHandler: v1.ProbeHandler{
												Exec: &v1.ExecAction{
													Command: []string{"/bin/sh", "-c", "/healthy.sh"},
												},
											},
										},
									},
								},
								{
									Name:          fmt.Sprintf("pod2-c2-%v", flagEliveness),
									ContainerName: "c2",
									Probe: appsv1alpha1.ContainerProbeSpec{
										Probe: v1.Probe{
											ProbeHandler: v1.ProbeHandler{
												Exec: &v1.ExecAction{
													Command: []string{"/bin/sh", "-c", "/healthy.sh"},
												},
											},
										},
									},
								},
							},
						},
						{
							Name:      "pod3",
							Namespace: "pod3-namespace",
							UID:       "333-222-333",
							Probes: []appsv1alpha1.ContainerProbe{
								{
									Name:          fmt.Sprintf("pod3-c1-%v", flagEliveness),
									ContainerName: "c1",
									Probe: appsv1alpha1.ContainerProbeSpec{
										Probe: v1.Probe{
											ProbeHandler: v1.ProbeHandler{
												Exec: &v1.ExecAction{
													Command: []string{"/bin/sh", "-c", "/healthy.sh"},
												},
											},
										},
									},
								},
								{
									Name:          "pod3-c2",
									ContainerName: "c2",
									Probe: appsv1alpha1.ContainerProbeSpec{
										Probe: v1.Probe{
											ProbeHandler: v1.ProbeHandler{
												HTTPGet: &v1.HTTPGetAction{
													Path:   "/index.html",
													Port:   intstr.IntOrString{Type: intstr.Int, StrVal: "9090"},
													Scheme: v1.URISchemeHTTP,
												},
											},
										},
									},
								},
							},
						},
						{
							Name:      "pod4",
							Namespace: "pod4-namespace",
							UID:       "444-222-333",
							Probes: []appsv1alpha1.ContainerProbe{
								{
									Name:          "pod4-c1",
									ContainerName: "c1",
									Probe: appsv1alpha1.ContainerProbeSpec{
										Probe: v1.Probe{
											ProbeHandler: v1.ProbeHandler{
												HTTPGet: &v1.HTTPGetAction{
													Path:   "/index.html",
													Port:   intstr.IntOrString{Type: intstr.Int, StrVal: "9090"},
													Scheme: v1.URISchemeHTTP,
												},
											},
										},
									},
								},
								{
									Name:          "pod4-c2",
									ContainerName: "c2",
									Probe: appsv1alpha1.ContainerProbeSpec{
										Probe: v1.Probe{
											ProbeHandler: v1.ProbeHandler{
												HTTPGet: &v1.HTTPGetAction{
													Path:   "/index.html",
													Port:   intstr.IntOrString{Type: intstr.Int, StrVal: "9090"},
													Scheme: v1.URISchemeHTTP,
												},
											},
										},
									},
								},
							},
						},
					},
				},
				Status: appsv1alpha1.NodePodProbeStatus{
					PodProbeStatuses: []appsv1alpha1.PodProbeStatus{
						{
							Name:      "pod1",
							Namespace: "pod1-namespace",
							UID:       "111-222-333",
							// exist a failed probe check result for pod1-c1
							ProbeStates: []appsv1alpha1.ContainerProbeState{
								{
									Name:  fmt.Sprintf("pod1-c1-%v", flagEliveness),
									State: appsv1alpha1.ProbeFailed,
								},
								{
									Name:  fmt.Sprintf("pod1-c2-%v", flagEliveness),
									State: appsv1alpha1.ProbeSucceeded,
								},
							},
						},
						{
							Name:      "pod2",
							Namespace: "pod2-namespace",
							UID:       "222-222-333",
							ProbeStates: []appsv1alpha1.ContainerProbeState{
								{
									Name:  fmt.Sprintf("pod2-c1-%v", flagEliveness),
									State: appsv1alpha1.ProbeFailed,
								},
								{
									Name:  fmt.Sprintf("pod2-c2-%v", flagEliveness),
									State: appsv1alpha1.ProbeSucceeded,
								},
							},
						},
						{
							Name:      "pod3",
							Namespace: "pod3-namespace",
							UID:       "333-222-333",
							ProbeStates: []appsv1alpha1.ContainerProbeState{
								{
									Name:  fmt.Sprintf("pod3-c1-%v", flagEliveness),
									State: appsv1alpha1.ProbeFailed,
								},
								{
									Name:  "pod3-c2",
									State: appsv1alpha1.ProbeSucceeded,
								},
							},
						},
						{
							Name:      "pod4",
							Namespace: "pod4-namespace",
							UID:       "444-222-333",
							ProbeStates: []appsv1alpha1.ContainerProbeState{
								{
									Name:  "pod4-c1",
									State: appsv1alpha1.ProbeSucceeded,
								},
								{
									Name:  "pod4-c2",
									State: appsv1alpha1.ProbeSucceeded,
								},
							},
						},
					},
				},
			},
			crrs: []appsv1alpha1.ContainerRecreateRequest{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "pod1-namespace",
						Labels: map[string]string{
							"owner":   controllerName,
							"usage":   "test-nodepodprobe-1",
							podUIDKey: "111-222-333",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								Name:       "test-nodepodprobe-1",
								Kind:       "NodePodProbe",
								Controller: &isController,
							},
						},
					},
					Spec: appsv1alpha1.ContainerRecreateRequestSpec{
						PodName: "pod1",
						Containers: []appsv1alpha1.ContainerRecreateRequestContainer{
							{
								Name: "c1",
							},
						},
						Strategy: &appsv1alpha1.ContainerRecreateRequestStrategy{
							FailurePolicy: appsv1alpha1.ContainerRecreateRequestFailurePolicyFail,
							ForceRecreate: true,
						},
						ActiveDeadlineSeconds:   &activeDeadlineSeconds,
						TTLSecondsAfterFinished: &ttLSecondsAfterFinished,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "pod2-namespace",
						Labels: map[string]string{
							"owner":   controllerName,
							"usage":   "test-nodepodprobe-1",
							podUIDKey: "222-222-333",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								Name:       "test-nodepodprobe-1",
								Kind:       "NodePodProbe",
								Controller: &isController,
							},
						},
					},
					Spec: appsv1alpha1.ContainerRecreateRequestSpec{
						PodName: "pod2",
						Containers: []appsv1alpha1.ContainerRecreateRequestContainer{
							{
								Name: "c1",
							},
						},
						Strategy: &appsv1alpha1.ContainerRecreateRequestStrategy{
							FailurePolicy: appsv1alpha1.ContainerRecreateRequestFailurePolicyFail,
							ForceRecreate: true,
						},
						ActiveDeadlineSeconds:   &activeDeadlineSeconds,
						TTLSecondsAfterFinished: &ttLSecondsAfterFinished,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "pod3-namespace",
						Labels: map[string]string{
							"owner":   controllerName,
							"usage":   "test-nodepodprobe-1",
							podUIDKey: "333-222-333",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								Name:       "test-nodepodprobe-1",
								Kind:       "NodePodProbe",
								Controller: &isController,
							},
						},
					},
					Spec: appsv1alpha1.ContainerRecreateRequestSpec{
						PodName: "pod3",
						Containers: []appsv1alpha1.ContainerRecreateRequestContainer{
							{
								Name: "c1",
							},
						},
						Strategy: &appsv1alpha1.ContainerRecreateRequestStrategy{
							FailurePolicy: appsv1alpha1.ContainerRecreateRequestFailurePolicyFail,
							ForceRecreate: true,
						},
						ActiveDeadlineSeconds:   &activeDeadlineSeconds,
						TTLSecondsAfterFinished: &ttLSecondsAfterFinished,
					},
				},
			},
			initPods: []*v1.Pod{
				{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Pod",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "pod1",
						Namespace:       "pod1-namespace",
						UID:             "111-222-333",
						ResourceVersion: "999",
						Annotations: map[string]string{
							alpha1.AnnotationUsingEnhancedLiveness: "true",
						},
					},
				},
				{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Pod",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "pod2",
						Namespace:       "pod2-namespace",
						UID:             "222-222-333",
						ResourceVersion: "999",
						Annotations: map[string]string{
							alpha1.AnnotationUsingEnhancedLiveness: "true",
						},
					},
				},
				{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Pod",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "pod3",
						Namespace:       "pod3-namespace",
						UID:             "333-222-333",
						ResourceVersion: "999",
						Annotations: map[string]string{
							alpha1.AnnotationUsingEnhancedLiveness: "true",
						},
					},
				},
				{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Pod",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "pod4",
						Namespace:       "pod4-namespace",
						UID:             "444-222-333",
						ResourceVersion: "999",
						Annotations: map[string]string{
							alpha1.AnnotationUsingEnhancedLiveness: "true",
						},
					},
				},
			},
			expectCRRs: &appsv1alpha1.ContainerRecreateRequestList{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ContainerRecreateRequestList",
					APIVersion: "apps.kruise.io/v1alpha1",
				},
				Items: []appsv1alpha1.ContainerRecreateRequest{
					{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: "pod1-namespace",
							Labels: map[string]string{
								"owner":   controllerName,
								"usage":   "test-nodepodprobe-1",
								podUIDKey: "111-222-333",
							},
							OwnerReferences: []metav1.OwnerReference{
								{
									Name:       "test-nodepodprobe-1",
									Kind:       "NodePodProbe",
									Controller: &isController,
								},
							},
						},
						Spec: appsv1alpha1.ContainerRecreateRequestSpec{
							PodName: "pod1",
							Containers: []appsv1alpha1.ContainerRecreateRequestContainer{
								{
									Name: "c1",
								},
							},
							Strategy: &appsv1alpha1.ContainerRecreateRequestStrategy{
								FailurePolicy: appsv1alpha1.ContainerRecreateRequestFailurePolicyFail,
								ForceRecreate: true,
							},
							ActiveDeadlineSeconds:   &activeDeadlineSeconds,
							TTLSecondsAfterFinished: &ttLSecondsAfterFinished,
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: "pod2-namespace",
							Labels: map[string]string{
								"owner":   controllerName,
								"usage":   "test-nodepodprobe-1",
								podUIDKey: "222-222-333",
							},
							OwnerReferences: []metav1.OwnerReference{
								{
									Name:       "test-nodepodprobe-1",
									Kind:       "NodePodProbe",
									Controller: &isController,
								},
							},
						},
						Spec: appsv1alpha1.ContainerRecreateRequestSpec{
							PodName: "pod2",
							Containers: []appsv1alpha1.ContainerRecreateRequestContainer{
								{
									Name: "c1",
								},
							},
							Strategy: &appsv1alpha1.ContainerRecreateRequestStrategy{
								FailurePolicy: appsv1alpha1.ContainerRecreateRequestFailurePolicyFail,
								ForceRecreate: true,
							},
							ActiveDeadlineSeconds:   &activeDeadlineSeconds,
							TTLSecondsAfterFinished: &ttLSecondsAfterFinished,
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: "pod3-namespace",
							Labels: map[string]string{
								"owner":   controllerName,
								"usage":   "test-nodepodprobe-1",
								podUIDKey: "333-222-333",
							},
							OwnerReferences: []metav1.OwnerReference{
								{
									Name:       "test-nodepodprobe-1",
									Kind:       "NodePodProbe",
									Controller: &isController,
								},
							},
						},
						Spec: appsv1alpha1.ContainerRecreateRequestSpec{
							PodName: "pod3",
							Containers: []appsv1alpha1.ContainerRecreateRequestContainer{
								{
									Name: "c1",
								},
							},
							Strategy: &appsv1alpha1.ContainerRecreateRequestStrategy{
								FailurePolicy: appsv1alpha1.ContainerRecreateRequestFailurePolicyFail,
								ForceRecreate: true,
							},
							ActiveDeadlineSeconds:   &activeDeadlineSeconds,
							TTLSecondsAfterFinished: &ttLSecondsAfterFinished,
						},
					},
				},
			},
		},
	}

	for _, tc := range testCase {
		t.Run(tc.name, func(t *testing.T) {
			initClientObjs := []client.Object{}
			for i := range tc.initPods {
				initClientObjs = append(initClientObjs, tc.initPods[i])
			}
			for i := range tc.crrs {
				initClientObjs = append(initClientObjs, &tc.crrs[i])
			}
			initClientObjs = append(initClientObjs, &tc.nodePodProbe)
			handler := ReconcileEnhancedLivenessProbe{
				Client: fake.NewClientBuilder().WithScheme(testscheme).
					WithObjects(initClientObjs...).Build(),
			}
			err := handler.syncNodePodProbe(context.TODO(), tc.nodePodProbe.Namespace, tc.nodePodProbe.Name)
			if err != nil {
				t.Errorf("Failed to sync nodePodProbe, err: %v", err)
			}

			getCRRsRelatedNodePodProbe, err := handler.listAllCRRsRelatedWithNodePodProbe(context.TODO(), tc.nodePodProbe)
			if err != nil {
				t.Errorf("Failed to list all crrs related with node pod probe, err: %v", err)
			}
			t.Logf("List all CRRs related with this nodePodProbe after doing syncNodePodProbe: %v", util.DumpJSON(getCRRsRelatedNodePodProbe))
			// for match the result, the name and resourceVersion and generationName should be empty.
			for i, _ := range getCRRsRelatedNodePodProbe.Items {
				getCRRsRelatedNodePodProbe.Items[i].Name = ""
				getCRRsRelatedNodePodProbe.Items[i].ResourceVersion = ""
				getCRRsRelatedNodePodProbe.Items[i].GenerateName = ""
			}
			if !reflect.DeepEqual(getCRRsRelatedNodePodProbe, tc.expectCRRs) {
				t.Errorf("No match, expect: %v, but: %v", util.DumpJSON(tc.expectCRRs), util.DumpJSON(getCRRsRelatedNodePodProbe))
			}
		})
	}

}

func TestListCRRObjectByPodUID(t *testing.T) {
	isController := true
	ttLSecondsAfterFinished := int32(600)
	activeDeadlineSeconds := int64(1800) // 30min = 60s * 30 = 1800s

	testCase := []struct {
		name             string
		crrs             []appsv1alpha1.ContainerRecreateRequest
		nodePodProbeName string
		namespace        string
		podUID           string
		expectCRRs       *appsv1alpha1.ContainerRecreateRequestList
	}{
		{
			name: "case1: list one CRR related with podUID",
			crrs: []appsv1alpha1.ContainerRecreateRequest{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "pod1-namespace",
						Labels: map[string]string{
							"owner":   controllerName,
							"usage":   "test-nodepodprobe-1",
							podUIDKey: "111-222-333",
						},
						ResourceVersion: "999",
						OwnerReferences: []metav1.OwnerReference{
							{
								Name:       "test-nodepodprobe-1",
								Kind:       "NodePodProbe",
								Controller: &isController,
							},
						},
					},
					Spec: appsv1alpha1.ContainerRecreateRequestSpec{
						PodName: "pod1",
						Containers: []appsv1alpha1.ContainerRecreateRequestContainer{
							{
								Name: "c1",
							},
						},
						Strategy: &appsv1alpha1.ContainerRecreateRequestStrategy{
							FailurePolicy: appsv1alpha1.ContainerRecreateRequestFailurePolicyFail,
							ForceRecreate: true,
						},
						ActiveDeadlineSeconds:   &activeDeadlineSeconds,
						TTLSecondsAfterFinished: &ttLSecondsAfterFinished,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "pod2-namespace",
						Labels: map[string]string{
							"owner":   controllerName,
							"usage":   "test-nodepodprobe-1",
							podUIDKey: "222-222-333",
						},
						ResourceVersion: "999",
						OwnerReferences: []metav1.OwnerReference{
							{
								Name:       "test-nodepodprobe-1",
								Kind:       "NodePodProbe",
								Controller: &isController,
							},
						},
					},
					Spec: appsv1alpha1.ContainerRecreateRequestSpec{
						PodName: "pod2",
						Containers: []appsv1alpha1.ContainerRecreateRequestContainer{
							{
								Name: "c1",
							},
						},
						Strategy: &appsv1alpha1.ContainerRecreateRequestStrategy{
							FailurePolicy: appsv1alpha1.ContainerRecreateRequestFailurePolicyFail,
							ForceRecreate: true,
						},
						ActiveDeadlineSeconds:   &activeDeadlineSeconds,
						TTLSecondsAfterFinished: &ttLSecondsAfterFinished,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "pod3-namespace",
						Labels: map[string]string{
							"owner":   controllerName,
							"usage":   "test-nodepodprobe-1",
							podUIDKey: "333-222-333",
						},
						ResourceVersion: "999",
						OwnerReferences: []metav1.OwnerReference{
							{
								Name:       "test-nodepodprobe-1",
								Kind:       "NodePodProbe",
								Controller: &isController,
							},
						},
					},
					Spec: appsv1alpha1.ContainerRecreateRequestSpec{
						PodName: "pod3",
						Containers: []appsv1alpha1.ContainerRecreateRequestContainer{
							{
								Name: "c1",
							},
						},
						Strategy: &appsv1alpha1.ContainerRecreateRequestStrategy{
							FailurePolicy: appsv1alpha1.ContainerRecreateRequestFailurePolicyFail,
							ForceRecreate: true,
						},
						ActiveDeadlineSeconds:   &activeDeadlineSeconds,
						TTLSecondsAfterFinished: &ttLSecondsAfterFinished,
					},
				},
			},
			nodePodProbeName: "test-nodepodprobe-1",
			namespace:        "pod3-namespace",
			podUID:           "333-222-333",
			expectCRRs: &appsv1alpha1.ContainerRecreateRequestList{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ContainerRecreateRequestList",
					APIVersion: "apps.kruise.io/v1alpha1",
				},
				Items: []appsv1alpha1.ContainerRecreateRequest{
					{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: "pod3-namespace",
							Labels: map[string]string{
								"owner":   controllerName,
								"usage":   "test-nodepodprobe-1",
								podUIDKey: "333-222-333",
							},
							ResourceVersion: "999",
							OwnerReferences: []metav1.OwnerReference{
								{
									Name:       "test-nodepodprobe-1",
									Kind:       "NodePodProbe",
									Controller: &isController,
								},
							},
						},
						Spec: appsv1alpha1.ContainerRecreateRequestSpec{
							PodName: "pod3",
							Containers: []appsv1alpha1.ContainerRecreateRequestContainer{
								{
									Name: "c1",
								},
							},
							Strategy: &appsv1alpha1.ContainerRecreateRequestStrategy{
								FailurePolicy: appsv1alpha1.ContainerRecreateRequestFailurePolicyFail,
								ForceRecreate: true,
							},
							ActiveDeadlineSeconds:   &activeDeadlineSeconds,
							TTLSecondsAfterFinished: &ttLSecondsAfterFinished,
						},
					},
				},
			},
		},
		{
			name: "case2: no found CRR related with podUID",
			crrs: []appsv1alpha1.ContainerRecreateRequest{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "pod1-namespace",
						Labels: map[string]string{
							"owner":   controllerName,
							"usage":   "test-nodepodprobe-1",
							podUIDKey: "111-222-333",
						},
						ResourceVersion: "999",
						OwnerReferences: []metav1.OwnerReference{
							{
								Name:       "test-nodepodprobe-1",
								Kind:       "NodePodProbe",
								Controller: &isController,
							},
						},
					},
					Spec: appsv1alpha1.ContainerRecreateRequestSpec{
						PodName: "pod1",
						Containers: []appsv1alpha1.ContainerRecreateRequestContainer{
							{
								Name: "c1",
							},
						},
						Strategy: &appsv1alpha1.ContainerRecreateRequestStrategy{
							FailurePolicy: appsv1alpha1.ContainerRecreateRequestFailurePolicyFail,
							ForceRecreate: true,
						},
						ActiveDeadlineSeconds:   &activeDeadlineSeconds,
						TTLSecondsAfterFinished: &ttLSecondsAfterFinished,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "pod2-namespace",
						Labels: map[string]string{
							"owner":   controllerName,
							"usage":   "test-nodepodprobe-1",
							podUIDKey: "222-222-333",
						},
						ResourceVersion: "999",
						OwnerReferences: []metav1.OwnerReference{
							{
								Name:       "test-nodepodprobe-1",
								Kind:       "NodePodProbe",
								Controller: &isController,
							},
						},
					},
					Spec: appsv1alpha1.ContainerRecreateRequestSpec{
						PodName: "pod2",
						Containers: []appsv1alpha1.ContainerRecreateRequestContainer{
							{
								Name: "c1",
							},
						},
						Strategy: &appsv1alpha1.ContainerRecreateRequestStrategy{
							FailurePolicy: appsv1alpha1.ContainerRecreateRequestFailurePolicyFail,
							ForceRecreate: true,
						},
						ActiveDeadlineSeconds:   &activeDeadlineSeconds,
						TTLSecondsAfterFinished: &ttLSecondsAfterFinished,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "pod3-namespace",
						Labels: map[string]string{
							"owner":   controllerName,
							"usage":   "test-nodepodprobe-1",
							podUIDKey: "333-222-333",
						},
						ResourceVersion: "999",
						OwnerReferences: []metav1.OwnerReference{
							{
								Name:       "test-nodepodprobe-1",
								Kind:       "NodePodProbe",
								Controller: &isController,
							},
						},
					},
					Spec: appsv1alpha1.ContainerRecreateRequestSpec{
						PodName: "pod3",
						Containers: []appsv1alpha1.ContainerRecreateRequestContainer{
							{
								Name: "c1",
							},
						},
						Strategy: &appsv1alpha1.ContainerRecreateRequestStrategy{
							FailurePolicy: appsv1alpha1.ContainerRecreateRequestFailurePolicyFail,
							ForceRecreate: true,
						},
						ActiveDeadlineSeconds:   &activeDeadlineSeconds,
						TTLSecondsAfterFinished: &ttLSecondsAfterFinished,
					},
				},
			},
			nodePodProbeName: "test-nodepodprobe-1",
			namespace:        "pod3-namespace",
			podUID:           "333-222-888",
			expectCRRs: &appsv1alpha1.ContainerRecreateRequestList{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ContainerRecreateRequestList",
					APIVersion: "apps.kruise.io/v1alpha1",
				},
				Items: []appsv1alpha1.ContainerRecreateRequest{},
			},
		},
		{
			name: "case3: found more than one CRR related with podUID",
			crrs: []appsv1alpha1.ContainerRecreateRequest{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "pod1-namespace",
						Name:      "pod1-crr-1",
						Labels: map[string]string{
							"owner":   controllerName,
							"usage":   "test-nodepodprobe-1",
							podUIDKey: "111-222-333",
						},
						ResourceVersion: "999",
						OwnerReferences: []metav1.OwnerReference{
							{
								Name:       "test-nodepodprobe-1",
								Kind:       "NodePodProbe",
								Controller: &isController,
							},
						},
					},
					Spec: appsv1alpha1.ContainerRecreateRequestSpec{
						PodName: "pod1",
						Containers: []appsv1alpha1.ContainerRecreateRequestContainer{
							{
								Name: "c1",
							},
						},
						Strategy: &appsv1alpha1.ContainerRecreateRequestStrategy{
							FailurePolicy: appsv1alpha1.ContainerRecreateRequestFailurePolicyFail,
							ForceRecreate: true,
						},
						ActiveDeadlineSeconds:   &activeDeadlineSeconds,
						TTLSecondsAfterFinished: &ttLSecondsAfterFinished,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "pod2-namespace",
						Name:      "pod2-crr-1",
						Labels: map[string]string{
							"owner":   controllerName,
							"usage":   "test-nodepodprobe-1",
							podUIDKey: "222-222-333",
						},
						ResourceVersion: "999",
						OwnerReferences: []metav1.OwnerReference{
							{
								Name:       "test-nodepodprobe-1",
								Kind:       "NodePodProbe",
								Controller: &isController,
							},
						},
					},
					Spec: appsv1alpha1.ContainerRecreateRequestSpec{
						PodName: "pod2",
						Containers: []appsv1alpha1.ContainerRecreateRequestContainer{
							{
								Name: "c1",
							},
						},
						Strategy: &appsv1alpha1.ContainerRecreateRequestStrategy{
							FailurePolicy: appsv1alpha1.ContainerRecreateRequestFailurePolicyFail,
							ForceRecreate: true,
						},
						ActiveDeadlineSeconds:   &activeDeadlineSeconds,
						TTLSecondsAfterFinished: &ttLSecondsAfterFinished,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "pod3-namespace",
						Name:      "pod3-crr-1",
						Labels: map[string]string{
							"owner":   controllerName,
							"usage":   "test-nodepodprobe-1",
							podUIDKey: "333-222-333",
						},
						ResourceVersion: "999",
						OwnerReferences: []metav1.OwnerReference{
							{
								Name:       "test-nodepodprobe-1",
								Kind:       "NodePodProbe",
								Controller: &isController,
							},
						},
					},
					Spec: appsv1alpha1.ContainerRecreateRequestSpec{
						PodName: "pod3",
						Containers: []appsv1alpha1.ContainerRecreateRequestContainer{
							{
								Name: "c1",
							},
						},
						Strategy: &appsv1alpha1.ContainerRecreateRequestStrategy{
							FailurePolicy: appsv1alpha1.ContainerRecreateRequestFailurePolicyFail,
							ForceRecreate: true,
						},
						ActiveDeadlineSeconds:   &activeDeadlineSeconds,
						TTLSecondsAfterFinished: &ttLSecondsAfterFinished,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "pod3-namespace",
						Name:      "pod3-crr-2",
						Labels: map[string]string{
							"owner":   controllerName,
							"usage":   "test-nodepodprobe-1",
							podUIDKey: "333-222-333",
						},
						ResourceVersion: "999",
						OwnerReferences: []metav1.OwnerReference{
							{
								Name:       "test-nodepodprobe-1",
								Kind:       "NodePodProbe",
								Controller: &isController,
							},
						},
					},
					Spec: appsv1alpha1.ContainerRecreateRequestSpec{
						PodName: "pod3",
						Containers: []appsv1alpha1.ContainerRecreateRequestContainer{
							{
								Name: "c1",
							},
						},
						Strategy: &appsv1alpha1.ContainerRecreateRequestStrategy{
							FailurePolicy: appsv1alpha1.ContainerRecreateRequestFailurePolicyFail,
							ForceRecreate: true,
						},
						ActiveDeadlineSeconds:   &activeDeadlineSeconds,
						TTLSecondsAfterFinished: &ttLSecondsAfterFinished,
					},
				},
			},
			nodePodProbeName: "test-nodepodprobe-1",
			namespace:        "pod3-namespace",
			podUID:           "333-222-333",
			expectCRRs: &appsv1alpha1.ContainerRecreateRequestList{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ContainerRecreateRequestList",
					APIVersion: "apps.kruise.io/v1alpha1",
				},
				Items: []appsv1alpha1.ContainerRecreateRequest{
					{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: "pod3-namespace",
							Name:      "pod3-crr-1",
							Labels: map[string]string{
								"owner":   controllerName,
								"usage":   "test-nodepodprobe-1",
								podUIDKey: "333-222-333",
							},
							ResourceVersion: "999",
							OwnerReferences: []metav1.OwnerReference{
								{
									Name:       "test-nodepodprobe-1",
									Kind:       "NodePodProbe",
									Controller: &isController,
								},
							},
						},
						Spec: appsv1alpha1.ContainerRecreateRequestSpec{
							PodName: "pod3",
							Containers: []appsv1alpha1.ContainerRecreateRequestContainer{
								{
									Name: "c1",
								},
							},
							Strategy: &appsv1alpha1.ContainerRecreateRequestStrategy{
								FailurePolicy: appsv1alpha1.ContainerRecreateRequestFailurePolicyFail,
								ForceRecreate: true,
							},
							ActiveDeadlineSeconds:   &activeDeadlineSeconds,
							TTLSecondsAfterFinished: &ttLSecondsAfterFinished,
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: "pod3-namespace",
							Name:      "pod3-crr-2",
							Labels: map[string]string{
								"owner":   controllerName,
								"usage":   "test-nodepodprobe-1",
								podUIDKey: "333-222-333",
							},
							ResourceVersion: "999",
							OwnerReferences: []metav1.OwnerReference{
								{
									Name:       "test-nodepodprobe-1",
									Kind:       "NodePodProbe",
									Controller: &isController,
								},
							},
						},
						Spec: appsv1alpha1.ContainerRecreateRequestSpec{
							PodName: "pod3",
							Containers: []appsv1alpha1.ContainerRecreateRequestContainer{
								{
									Name: "c1",
								},
							},
							Strategy: &appsv1alpha1.ContainerRecreateRequestStrategy{
								FailurePolicy: appsv1alpha1.ContainerRecreateRequestFailurePolicyFail,
								ForceRecreate: true,
							},
							ActiveDeadlineSeconds:   &activeDeadlineSeconds,
							TTLSecondsAfterFinished: &ttLSecondsAfterFinished,
						},
					},
				},
			},
		},
	}
	for _, tc := range testCase {
		t.Run(tc.name, func(t *testing.T) {
			initClientObjs := []client.Object{}
			for i := range tc.crrs {
				initClientObjs = append(initClientObjs, &tc.crrs[i])
			}
			handler := ReconcileEnhancedLivenessProbe{
				Client: fake.NewClientBuilder().WithScheme(testscheme).
					WithObjects(initClientObjs...).Build(),
			}
			got, err := handler.listCRRObjectByPodUID(context.TODO(), tc.nodePodProbeName, tc.namespace, tc.podUID)
			if err != nil {
				t.Errorf("Failed to list crrs object by pod uid, err: %v", err)
			}
			if !reflect.DeepEqual(got, tc.expectCRRs) {
				t.Errorf("No match, expect: %v, but: %v", util.DumpJSON(tc.expectCRRs), util.DumpJSON(got))
			}
		})
	}
}

func TestExistPodProbeInNodePodProbe(t *testing.T) {
	testCase := []struct {
		name         string
		nodePodProbe appsv1alpha1.NodePodProbe
		podNamespace string
		podName      string
		podUID       string
		expectGot    bool
	}{
		{
			name: "case1: exist pod probe in nodePodProbe",
			nodePodProbe: appsv1alpha1.NodePodProbe{
				Spec: appsv1alpha1.NodePodProbeSpec{
					PodProbes: []appsv1alpha1.PodProbe{
						{
							Name:      "pod1",
							Namespace: "pod1-namespace",
							UID:       "111-222-333",
							IP:        "1.1.1.1",
							Probes: []appsv1alpha1.ContainerProbe{
								{
									Name:          "pod1-c1",
									ContainerName: "c1",
								},
							},
						},
						{
							Name:      "pod2",
							Namespace: "pod2-namespace",
							UID:       "222-222-333",
							IP:        "2.2.2.2",
							Probes: []appsv1alpha1.ContainerProbe{
								{
									Name:          "pod2-c1",
									ContainerName: "c1",
								},
							},
						},
					},
				},
			},
			podNamespace: "pod2-namespace",
			podName:      "pod2",
			podUID:       "222-222-333",
			expectGot:    true,
		},

		{
			name: "case2: no Found pod probe in nodePodProbe",
			nodePodProbe: appsv1alpha1.NodePodProbe{
				Spec: appsv1alpha1.NodePodProbeSpec{
					PodProbes: []appsv1alpha1.PodProbe{
						{
							Name:      "pod1",
							Namespace: "pod1-namespace",
							UID:       "111-222-333",
							IP:        "1.1.1.1",
							Probes: []appsv1alpha1.ContainerProbe{
								{
									Name:          "pod1-c1",
									ContainerName: "c1",
								},
							},
						},
						{
							Name:      "pod2",
							Namespace: "pod2-namespace",
							UID:       "222-222-333",
							IP:        "2.2.2.2",
							Probes: []appsv1alpha1.ContainerProbe{
								{
									Name:          "pod2-c1",
									ContainerName: "c1",
								},
							},
						},
					},
				},
			},
			podNamespace: "pod3-namespace",
			podName:      "pod3",
			podUID:       "222-222-333",
			expectGot:    false,
		},
	}

	for _, tc := range testCase {
		got := existPodProbeInNodePodProbe(tc.nodePodProbe, tc.podNamespace, tc.podName, tc.podUID)
		if !reflect.DeepEqual(got, tc.expectGot) {
			t.Errorf("No match, expect: %v, but: %v", tc.expectGot, got)
		}
	}
}

/*
Copyright 2022 The Kruise Authors.

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

package containerexitpriority

import (
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"math/rand"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	daemonruntime "github.com/openkruise/kruise/pkg/daemon/criruntime"
	daemonoptions "github.com/openkruise/kruise/pkg/daemon/options"
	daemonutil "github.com/openkruise/kruise/pkg/daemon/util"
	"github.com/openkruise/kruise/pkg/features"
	"github.com/openkruise/kruise/pkg/util"
	"github.com/openkruise/kruise/pkg/util/containerexitpriority"
	utilfeature "github.com/openkruise/kruise/pkg/util/feature"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/util/workqueue"
	criapi "k8s.io/cri-api/pkg/apis"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
	"k8s.io/klog/v2"
	kubeletcontainer "k8s.io/kubernetes/pkg/kubelet/container"
	kubelettypes "k8s.io/kubernetes/pkg/kubelet/types"
)

var (
	workers         = 3
	sharedPodVolume = path.Join(containerexitpriority.KruiseDaemonShareRootVolume, "pods")
	// kruise entrypoint binary
	entrypointBinary = "entrypoint"
	// compile, for example: /var/lib/kruise-daemon/pods/{uid}/{container.name}
	containerDirCompile = regexp.MustCompile(fmt.Sprintf("^%s/[a-zA-Z0-9_-]+/[a-zA-Z0-9_-]+\\.*[a-zA-Z0-9_-]*$", sharedPodVolume))
	// compile, for example: /var/lib/kruise-daemon/pods/{uid}/{container.name}/exit
	exitFileCompile = regexp.MustCompile(fmt.Sprintf("^%s/[a-zA-Z0-9_-]+/[a-zA-Z0-9_-]+\\.*[a-zA-Z0-9_-]+/%s$", sharedPodVolume, containerexitpriority.KruiseContainerExitPriorityFile))
	// compile, for example: /var/lib/kruise-daemon/pods/{uid}
	podDirCompile = regexp.MustCompile(fmt.Sprintf("^%s/[a-zA-Z0-9_-]+$", sharedPodVolume))
)

type Controller struct {
	exitPriorityQueue workqueue.RateLimitingInterface
	installQueue      workqueue.RateLimitingInterface
	podLister         corelisters.PodLister
	runtimeFactory    daemonruntime.Factory
}

// NewController returns the Controller for container-exit-priority reporting
func NewController(opts daemonoptions.Options) (*Controller, error) {
	c := &Controller{
		exitPriorityQueue: workqueue.NewNamedRateLimitingQueue(
			// Backoff duration from 500ms to 50~55s
			workqueue.NewItemExponentialFailureRateLimiter(500*time.Millisecond, 50*time.Second+time.Millisecond*time.Duration(rand.Intn(5000))),
			"container_exit_priority",
		),
		installQueue: workqueue.NewNamedRateLimitingQueue(
			// Backoff duration from 500ms to 50~55s
			workqueue.NewItemExponentialFailureRateLimiter(500*time.Millisecond, 50*time.Second+time.Millisecond*time.Duration(rand.Intn(5000))),
			"install_entrypoint",
		),
	}
	if !utilfeature.DefaultFeatureGate.Enabled(features.DaemonAccessKubeletApi) {
		if opts.PodInformer == nil {
			return nil, fmt.Errorf("container-exit-priority Controller can not run without pod informer")
		}
		c.podLister = corelisters.NewPodLister(opts.PodInformer.GetIndexer())
	}
	c.runtimeFactory = opts.RuntimeFactory
	return c, nil
}

func (c *Controller) Run(stop <-chan struct{}) {
	klog.Infof("Starting container exit priority controller")
	err := os.MkdirAll(sharedPodVolume, 0644)
	if err != nil {
		panic(err)
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		panic(err)
	}
	// watch /var/lib/kruise-daemon/pods
	err = c.watchAndQueuePath(watcher, sharedPodVolume)
	if err != nil {
		panic(err)
	}
	defer utilruntime.HandleCrash()
	defer c.exitPriorityQueue.ShutDown()
	defer c.installQueue.ShutDown()
	defer watcher.Close()

	// cleanup deleted pod share volume
	go c.cleanupShareVolume()
	// handler exit priority
	go c.watchExitPriorityEvent(watcher)
	for i := 0; i < workers; i++ {
		go wait.Until(func() {
			for c.processNextExitPriorityItem() {
			}
		}, time.Second, stop)
		go wait.Until(func() {
			for c.processNextInstallEntrypointItem() {
			}
		}, time.Second, stop)

	}
	klog.Info("Started containerExitPriority controller successfully")
	<-stop
}

// watch the folder and sub folders
func (c *Controller) watchAndQueuePath(watcher *fsnotify.Watcher, dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			klog.Warningf("containerExitPriority walk dir(%s) failed: %s", path, err.Error())
			return err
		}
		if info.IsDir() {
			err = watcher.Add(path)
			if err != nil {
				return err
			}
			c.queueInstallEntrypointBinary(path)
		} else {
			c.queueContainerExitEvent(path)
		}
		return nil
	})
}

func (c *Controller) queueInstallEntrypointBinary(dir string) {
	values := strings.Split(dir, "/")
	if !containerDirCompile.MatchString(dir) || len(values) != 7 {
		return
	}
	podUid := values[5]
	pod, err := c.fetchPodForUid(podUid)
	if err != nil || pod == nil || pod.Annotations[containerexitpriority.AutoInjectContainerExitPriorityPreStopAnnotation] != "true" {
		return
	}
	c.installQueue.Add(dir)
	klog.Infof("containerExitPriority queue install entrypoint(%s)", dir)
}

func (c *Controller) installEntrypointBinary(dir string) error {
	target := path.Join(dir, entrypointBinary)
	if _, err := os.Stat(target); err == nil {
		return nil
	}
	source, err := os.Open(path.Join("/", entrypointBinary))
	if err != nil {
		klog.Errorf("containerExitPriority open /%s failed: %s", entrypointBinary, err.Error())
		return err
	}
	defer source.Close()
	destination, err := os.OpenFile(target, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		klog.Errorf("containerExitPriority create /%s failed: %s", target, err.Error())
		return err
	}
	defer destination.Close()

	ch := make(chan struct{}, 1)
	go func() {
		_, err = io.Copy(destination, source)
		ch <- struct{}{}
	}()
	select {
	case <-ch:
		if err != nil {
			klog.Errorf("containerExitPriority install file(%s) failed: %s", target, err.Error())
			return err
		}
	case <-time.After(30 * time.Second):
		return fmt.Errorf("containerExitPriority install file(%s) timeout(30s)", target)
	}
	klog.Infof("containerExitPriority install entrypoint(%s) success", target)
	return nil
}

func (c *Controller) watchExitPriorityEvent(watcher *fsnotify.Watcher) {
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				klog.Errorf("containerExitPriority fs-notify(%s) failed", sharedPodVolume)
				return
			}
			if event.Op&fsnotify.Create == fsnotify.Create {
				klog.Infof("containerExitPriority watch event(%v)", event)
				c.watchAndQueuePath(watcher, event.Name)
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				klog.Errorf("containerExitPriority fs-notify(%s) failed", sharedPodVolume)
				return
			}
			klog.Errorf("containerExitPriority fs-notify(%s) failed: %s", sharedPodVolume, err.Error())
		}
	}
}

func (c *Controller) queueContainerExitEvent(file string) {
	values := strings.Split(file, "/")
	if !exitFileCompile.MatchString(file) || len(values) != 8 {
		return
	}
	podUid := values[5]
	c.exitPriorityQueue.Add(podUid)
	klog.Infof("containerExitPriority fs-notify(%s), and queue %s", file, podUid)
}

// processNextExitPriorityItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNextExitPriorityItem() bool {
	// pull the next work item from queue.  It should be a key we use to lookup
	// something in a cache
	key, quit := c.exitPriorityQueue.Get()
	if quit {
		return false
	}
	defer c.exitPriorityQueue.Done(key)

	err := c.sync(key.(string))
	if err == nil {
		// No error, tell the queue to stop tracking history
		c.exitPriorityQueue.Forget(key)
	} else {
		// requeue the item to work on later
		c.exitPriorityQueue.AddAfter(key, time.Second)
	}
	return true
}

// processNextInstallEntrypointItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNextInstallEntrypointItem() bool {
	// pull the next work item from queue.  It should be a key we use to lookup
	// something in a cache
	key, quit := c.installQueue.Get()
	if quit {
		return false
	}
	defer c.installQueue.Done(key)

	err := c.installEntrypointBinary(key.(string))
	if err == nil {
		// No error, tell the queue to stop tracking history
		c.installQueue.Forget(key)
	} else {
		// requeue the item to work on later
		c.installQueue.AddAfter(key, time.Second)
	}
	return true
}

// uid is pod.UID
func (c *Controller) sync(uid string) (retErr error) {
	pod, err := c.fetchPodForUid(uid)
	if err != nil {
		return err
	} else if pod == nil {
		return nil
	}
	return c.handlerContainerExitPriority(pod)
}

func (c *Controller) fetchPodForUid(uid string) (*corev1.Pod, error) {
	pods, err := c.listNodePods()
	if err != nil {
		klog.Errorf("ContainerExitPriority Controller list pod failed: %s", err)
		return nil, err
	}
	for i := range pods {
		pod := pods[i]
		if string(pod.UID) == uid {
			return pod, nil
		}
	}
	klog.Warningf("ContainerExitPriority can not find pod(%s)", uid)
	return nil, nil
}

func (c *Controller) handlerContainerExitPriority(pod *corev1.Pod) error {
	exitPriorityContainers := containerexitpriority.ListExitPriorityContainers(pod)
	// in-place update sidecar container, no need to execute exitPriority
	if pod.DeletionTimestamp.IsZero() {
		for _, container := range exitPriorityContainers {
			if err := allowContainerExit(string(pod.UID), container.Name); err != nil {
				klog.Errorf("ContainerExirPriority allow pod(%s/%s) container(%s) exit failed: %s", pod.Namespace, pod.Name, container.Name, err.Error())
				return err
			}
		}
		klog.Infof("ContainerExitPriority pod(%s/%s) is in-place update, and allow sidecar containers exit done", pod.Namespace, pod.Name)
		return nil
	}

	// runtimeService, for example docker
	runtimeService := c.getRuntimeService(pod.Status.ContainerStatuses)
	if runtimeService == nil {
		klog.Infof("ContainerExitPriority pod(%s/%s) not found runtimeService", pod.Namespace, pod.Name)
		return nil
	}
	containers, err := runtimeService.ListContainers(&runtimeapi.ContainerFilter{
		LabelSelector: map[string]string{kubelettypes.KubernetesPodUIDLabel: string(pod.UID)},
	})
	if err != nil {
		klog.Errorf("ContainerExitPriority pod(%s/%s) list containers failed: %s", pod.Namespace, pod.Name, err.Error())
		return err
	}
	cStatus := map[string]struct {
		CreateAt int64
		State    runtimeapi.ContainerState
	}{}
	// Because if the container has been restarted, there may be multiple runtimes for the same container
	for i := range containers {
		container := containers[i]
		if obj, ok := cStatus[container.Metadata.Name]; !ok {
			cStatus[container.Metadata.Name] = struct {
				CreateAt int64
				State    runtimeapi.ContainerState
			}{container.CreatedAt, container.State}
			// select latest container
		} else if container.CreatedAt > obj.CreateAt {
			cStatus[container.Metadata.Name] = struct {
				CreateAt int64
				State    runtimeapi.ContainerState
			}{container.CreatedAt, container.State}
		}
	}
	klog.Infof("ContainerExitPriority pod(%s/%s) list container status(%s), and exitPriority(%s)", pod.Namespace, pod.Name,
		util.DumpJSON(cStatus), util.DumpJSON(exitPriorityContainers))
	// delete pod scenario
	wRunningContainers := sets.NewString()
	for _, container := range exitPriorityContainers {
		if cStatus[container.Name].State != runtimeapi.ContainerState_CONTAINER_RUNNING {
			continue
		}
		allowedExit := true
		for _, wContainer := range container.WaitContainers {
			if obj, ok := cStatus[wContainer]; ok && obj.State == runtimeapi.ContainerState_CONTAINER_RUNNING {
				wRunningContainers.Insert(wContainer)
				allowedExit = false
				break
			}
		}
		if !allowedExit {
			continue
		}
		// allow sidecar containers exit
		if err = allowContainerExit(string(pod.UID), container.Name); err != nil {
			klog.Errorf("ContainerExitPriority allow pod(%s/%s) container(%s) exit failed: %s", pod.Namespace, pod.Name, container.Name, err.Error())
			return err
		}
		klog.Infof("ContainerExitPriority allow pod(%s/%s) container(%s) exit", pod.Namespace, pod.Name, container.Name)
	}
	if wRunningContainers.Len() > 0 {
		err = fmt.Errorf("wait containers(%v) exit", wRunningContainers.List())
		klog.Infof("ContainerExitPriority wait pod(%s/%s) containers(%s) exit", pod.Namespace, pod.Name, wRunningContainers.List())
		return err
	}
	klog.Infof("ContainerExitPriority handler pod(%s/%s) sidecar containers exit done", pod.Namespace, pod.Name)
	return nil
}

func (c *Controller) getRuntimeService(cStatus []corev1.ContainerStatus) criapi.RuntimeService {
	for _, status := range cStatus {
		if status.ContainerID != "" {
			containerID := kubeletcontainer.ContainerID{}
			if err := containerID.ParseString(status.ContainerID); err != nil {
				klog.Errorf("failed to parse containerID %s: %v", status.ContainerID, err)
				return nil
			} else if containerID.Type == "" {
				klog.Errorf("no runtime name in containerID %s", status.ContainerID)
				return nil
			}
			return c.runtimeFactory.GetRuntimeServiceByName(containerID.Type)
		}
	}
	return nil
}

func (c *Controller) cleanupShareVolume() {
	for {
		podList, err := c.listNodePods()
		if err != nil {
			klog.Errorf("ContainerExitPriority Controller list pod failed: %s", err)
			time.Sleep(time.Second)
			continue
		}
		uids := map[string]struct{}{}
		for _, pod := range podList {
			uids[string(pod.UID)] = struct{}{}
		}
		filepath.Walk(sharedPodVolume, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() && podDirCompile.MatchString(path) {
				values := strings.Split(path, "/")
				if len(values) != 6 {
					return nil
				}
				if _, ok := uids[values[5]]; ok {
					return nil
				}
				// cleanup deleted pod share volume
				klog.Infof("containerExitPriority start cleanup share volume(%s)...", path)
				err = os.RemoveAll(path)
				if err != nil {
					klog.Errorf("containerExitPriority cleanup share volume(%s) failed: %s", path, err.Error())
					return err
				}
				klog.Infof("containerExitPriority cleanup share volume(%s) success", path)
			}
			return nil
		})

		time.Sleep(time.Minute)
	}
}

func (c *Controller) listNodePods() ([]*corev1.Pod, error) {
	// list pods against kubelet api.
	if utilfeature.DefaultFeatureGate.Enabled(features.DaemonAccessKubeletApi) {
		podList, err := daemonutil.GetNodePods()
		if err != nil {
			return nil, err
		}
		pods := make([]*corev1.Pod, 0, len(podList.Items))
		for i := range podList.Items {
			pod := &podList.Items[i]
			// csk 场景，优先使用 vc 中Pod的UID
			if pod.Annotations["tenancy.x-k8s.io/uid"] != "" {
				pod.UID = types.UID(pod.Annotations["tenancy.x-k8s.io/uid"])
			}
			pods = append(pods, pod)
		}
		return pods, nil
	}

	// list pods against k8s api-server
	return c.podLister.List(labels.Everything())
}

func allowContainerExit(podUid, container string) error {
	ch := make(chan struct{}, 1)
	file := path.Join(sharedPodVolume, podUid, container, containerexitpriority.KruiseContainerExitPriorityFile)
	var err error
	var by []byte
	go func() {
		by, err = ioutil.ReadFile(file)
		ch <- struct{}{}
	}()
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	select {
	case <-ch:
		if string(by) == containerexitpriority.KruiseContainerAllowExitFlag {
			return nil
		}
		return ioutil.WriteFile(file, []byte(containerexitpriority.KruiseContainerAllowExitFlag), fs.ModeNamedPipe)
	case <-time.After(10 * time.Second):
		return fmt.Errorf("containerExitPriority read file(%s) timeout", file)
	}
}

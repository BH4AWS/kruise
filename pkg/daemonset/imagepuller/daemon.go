/*
Copyright 2019 The Kruise Authors.

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

package imagepuller

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"reflect"
	"time"

	"golang.org/x/time/rate"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	clientset "k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/tools/reference"
	"k8s.io/client-go/util/retry"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"

	appsv1alpha1 "github.com/openkruise/kruise/apis/apps/v1alpha1"
	clientsetalpha1 "github.com/openkruise/kruise/pkg/client/clientset/versioned"
	clientalpha1 "github.com/openkruise/kruise/pkg/client/clientset/versioned/typed/apps/v1alpha1"
	listersalpha1 "github.com/openkruise/kruise/pkg/client/listers/apps/v1alpha1"
	imageruntime "github.com/openkruise/kruise/pkg/daemonset/imagepuller/runtime"
	pullerutil "github.com/openkruise/kruise/pkg/daemonset/imagepuller/util"
	"github.com/openkruise/kruise/pkg/util"
)

var (
	containerRuntimeType string
	containerRuntimeURI  string
	scheme               = runtime.NewScheme()
)

func init() {
	flag.StringVar(&containerRuntimeType, "container-runtime-type", "docker", "Type of container runtime")
	flag.StringVar(&containerRuntimeURI, "container-runtime-uri", "unix:///var/run/docker.sock", "URI of container runtime")
	appsv1alpha1.AddToScheme(scheme)
}

// Daemon is interface for process to run every node
type Daemon interface {
	HealthzHandler() func(http.ResponseWriter, *http.Request)
	Run(stopCh <-chan struct{}) error
}

type daemon struct {
	nodeName              string
	queue                 workqueue.RateLimitingInterface
	puller                puller
	imagePullNodeInformer cache.SharedIndexInformer
	imagePullNodeLister   listersalpha1.NodeImageLister
	healthz               *Healthz
	kubeClient            clientset.Interface
	eventRecorder         record.EventRecorder
	statusUpdater         *statusUpdater
	cacheSynced           bool
}

// NewDaemon create a imagepuller daemon
func NewDaemon(cfg *rest.Config) (Daemon, error) {
	nodeName := os.Getenv("NODE_NAME")
	if len(nodeName) == 0 {
		return nil, fmt.Errorf("failed to new daemon: NODE_NAME env is empty")
	}

	kubeClient := clientset.NewForConfigOrDie(cfg)
	clientset := clientsetalpha1.NewForConfigOrDie(cfg)
	informer := newNodeImageInformer(clientset, nodeName)
	secretManager := pullerutil.NewCacheBasedSecretManager(kubeClient)

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&corev1.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})

	recorder := eventBroadcaster.NewRecorder(scheme, v1.EventSource{Component: "imagepuller", Host: nodeName})
	accountManager := pullerutil.NewImagePullAccountManager(kubeClient)
	imageRuntime, err := imageruntime.NewImageRuntime(containerRuntimeType, containerRuntimeURI, accountManager)
	if err != nil {
		return nil, fmt.Errorf("failed to create container routime: %v", err)
	}
	puller, err := newRealPuller(imageRuntime, secretManager, recorder)
	if err != nil {
		return nil, fmt.Errorf("failed to new puller: %v", err)
	}

	d := &daemon{
		nodeName: nodeName,
		queue: workqueue.NewNamedRateLimitingQueue(
			workqueue.NewItemExponentialFailureRateLimiter(500*time.Millisecond, 300*time.Second), // 从0.5s开始重试，稳重一点
			"imagepuller",
		),
		puller:                puller,
		kubeClient:            kubeClient,
		eventRecorder:         recorder,
		statusUpdater:         newStatusUpdater(clientset.AppsV1alpha1().NodeImages()),
		imagePullNodeInformer: informer,
		imagePullNodeLister:   listersalpha1.NewNodeImageLister(informer.GetIndexer()),
		healthz:               NewHealthz(),
	}

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			node, ok := obj.(*appsv1alpha1.NodeImage)
			if ok {
				d.enqueue(node)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldNodeImage, oldOK := oldObj.(*appsv1alpha1.NodeImage)
			newNodeImage, newOK := newObj.(*appsv1alpha1.NodeImage)
			if !oldOK || !newOK {
				return
			}
			if reflect.DeepEqual(oldNodeImage.Spec, newNodeImage.Spec) {
				klog.V(4).Infof("Find imagePullNode %s spec has not changed, skip enqueueing.", newNodeImage.Name)
				return
			}
			d.logNewImages(oldNodeImage, newNodeImage)
			d.enqueue(newNodeImage)
		},
	})

	return d, nil
}

func (d *daemon) logNewImages(oldObj, newObj *appsv1alpha1.NodeImage) {
	oldImages := make(map[string]struct{})
	if oldObj != nil {
		for image, imageSpec := range oldObj.Spec.Images {
			for _, tagSpec := range imageSpec.Tags {
				fullName := fmt.Sprintf("%v:%v", image, tagSpec.Tag)
				oldImages[fullName] = struct{}{}
			}
		}
	}

	for image, imageSpec := range newObj.Spec.Images {
		for _, tagSpec := range imageSpec.Tags {
			fullName := fmt.Sprintf("%v:%v", image, tagSpec.Tag)
			if _, ok := oldImages[fullName]; !ok {
				klog.V(2).Infof("Received new image %v", fullName)
			}
		}
	}
}

func (d *daemon) HealthzHandler() func(http.ResponseWriter, *http.Request) {
	return d.healthz.Handler
}

func (d *daemon) Run(stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer d.queue.ShutDown()

	d.healthz.RegisterFunc("daemonCheck", func(req *http.Request) error {
		if !d.cacheSynced {
			return fmt.Errorf("cacheNotReady")
		}
		return nil
	})

	klog.Info("Starting informer for NodeImage")
	go d.imagePullNodeInformer.Run(stopCh)
	if !cache.WaitForCacheSync(stopCh, d.imagePullNodeInformer.HasSynced) {
		return fmt.Errorf("timed out waiting for caches to sync")
	}
	d.cacheSynced = true

	klog.Infof("Starting imagepuller")
	// Launch one workers to process resources, for there is only one NodeImage per Node
	go wait.Until(func() {
		for d.processNextWorkItem() {
		}
	}, time.Second, stopCh)

	klog.Info("Started successfully")
	<-stopCh
	klog.Info("Shutting down daemon")
	return nil
}

func newNodeImageInformer(client clientsetalpha1.Interface, nodeName string) cache.SharedIndexInformer {
	tweakListOptionsFunc := func(opt *metav1.ListOptions) {
		opt.FieldSelector = "metadata.name=" + nodeName
	}

	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				tweakListOptionsFunc(&options)
				return client.AppsV1alpha1().NodeImages().List(context.TODO(), options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				tweakListOptionsFunc(&options)
				return client.AppsV1alpha1().NodeImages().Watch(context.TODO(), options)
			},
		},
		&appsv1alpha1.NodeImage{},
		time.Hour*12,
		cache.Indexers{},
	)
}

func (d *daemon) enqueue(imagePullNode *appsv1alpha1.NodeImage) {
	if imagePullNode.DeletionTimestamp != nil {
		return
	}

	key, err := cache.MetaNamespaceKeyFunc(imagePullNode)
	if err != nil {
		klog.Warningf("Failed to get meta key for %s", imagePullNode.Name)
		return
	}
	d.queue.Add(key)
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (d *daemon) processNextWorkItem() bool {
	// pull the next work item from queue.  It should be a key we use to lookup
	// something in a cache
	key, quit := d.queue.Get()
	if quit {
		return false
	}
	defer d.queue.Done(key)

	err := d.sync(key.(string))

	if err == nil {
		// No error, tell the queue to stop tracking history
		d.queue.Forget(key)
	} else {
		// requeue the item to work on later
		d.queue.AddRateLimited(key)
	}

	return true
}

func (d *daemon) sync(key string) (retErr error) {
	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		klog.Warningf("Invalid key: %s", key)
		return nil
	}

	imagePullNode, err := d.imagePullNodeLister.Get(name)
	if errors.IsNotFound(err) {
		return nil
	} else if err != nil {
		klog.Errorf("Failed to get %s imagePullNode: %v", name, err)
		return err
	}

	klog.V(3).Infof("Start syncing for %s", name)

	defer func() {
		if retErr != nil {
			klog.Errorf("Failed to sync for %s: %v", name, retErr)
		} else {
			klog.V(3).Infof("Finished syncing for %s", name)
		}
	}()

	newStatus := appsv1alpha1.NodeImageStatus{
		ImageStatuses: make(map[string]appsv1alpha1.ImageStatus),
	}

	ref, _ := reference.GetReference(scheme, imagePullNode)
	retErr = d.puller.Sync(&imagePullNode.Spec, ref)
	if retErr != nil {
		return
	}

	for imageName, imageSpec := range imagePullNode.Spec.Images {
		newStatus.Desired += int32(len(imageSpec.Tags))

		imageStatuses := d.puller.GetStatus(imageName)
		if klog.V(9) {
			klog.V(9).Infof("get image %v status %#v", imageName, imageStatuses)
		}
		if imageStatuses == nil {
			continue
		}
		newStatus.ImageStatuses[imageName] = *imageStatuses
		for _, tagStatus := range imageStatuses.Tags {
			switch tagStatus.Phase {
			case appsv1alpha1.ImagePhaseSucceeded:
				newStatus.Succeeded++
			case appsv1alpha1.ImagePhaseFailed:
				newStatus.Failed++
			case appsv1alpha1.ImagePhasePulling:
				newStatus.Pulling++
			}
		}
	}
	if len(newStatus.ImageStatuses) == 0 {
		newStatus.ImageStatuses = nil
	}

	retErr = retry.RetryOnConflict(retry.DefaultBackoff, func() (err error) {
		// try to get new data
		imagePullNode, err = d.imagePullNodeLister.Get(name)
		if errors.IsNotFound(err) {
			klog.Infof("node %v not found, skip", name)
			return nil
		} else if err != nil {
			klog.Errorf("Failed to get %s imagePullNode: %v", name, err)
			return err
		}
		err = d.statusUpdater.updateStatus(imagePullNode, newStatus)
		return err
	})
	if retErr != nil {
		return retErr
	}

	// 如果当前有任务在执行，则1~2s触发一次reconcile
	// 如果当前没有拉取任务在执行，则50~60s触发一次reconcile
	if d.isImageInPulling(imagePullNode.Spec, newStatus) {
		d.queue.AddAfter(key, time.Second+time.Millisecond*time.Duration(rand.Intn(1000)))
	} else {
		d.queue.AddAfter(key, 50*time.Second+time.Millisecond*time.Duration(rand.Intn(10000)))
	}
	return nil
}

func (d *daemon) isImageInPulling(spec appsv1alpha1.NodeImageSpec, status appsv1alpha1.NodeImageStatus) bool {
	if status.Succeeded+status.Failed < status.Desired {
		return true
	}

	tagSpecs := make(map[string]appsv1alpha1.ImageTagSpec)
	for image, imageSpec := range spec.Images {
		for _, tagSpec := range imageSpec.Tags {
			fullName := fmt.Sprintf("%v:%v", image, tagSpec.Tag)
			tagSpecs[fullName] = tagSpec
		}
	}
	for image, imageStatus := range status.ImageStatuses {
		for _, tagStatus := range imageStatus.Tags {
			fullName := fmt.Sprintf("%v:%v", image, tagStatus.Tag)
			if tagSpec, ok := tagSpecs[fullName]; ok && tagSpec.Version != tagStatus.Version {
				return true
			}
		}
	}

	return false
}

type statusUpdater struct {
	imagePullNodeClient clientalpha1.NodeImageInterface

	previousTimestamp time.Time
	previousStatus    *appsv1alpha1.NodeImageStatus
	rateLimiter       *rate.Limiter
}

const (
	statusUpdateQPS   = 0.2
	statusUpdateBurst = 5
)

func newStatusUpdater(imagePullNodeClient clientalpha1.NodeImageInterface) *statusUpdater {
	return &statusUpdater{
		imagePullNodeClient: imagePullNodeClient,
		previousStatus:      &appsv1alpha1.NodeImageStatus{},
		previousTimestamp:   time.Now().Add(-time.Hour * 24),
		rateLimiter:         rate.NewLimiter(statusUpdateQPS, statusUpdateBurst),
	}
}

func (su *statusUpdater) updateStatus(imagePullNode *appsv1alpha1.NodeImage, newStatus appsv1alpha1.NodeImageStatus) error {
	// IMPORTANT!!! 必须要控制上报频率，否则异常场景下所有daemon都在疯狂上报，apiserver就跪了!!!
	if !su.statusChanaged(&newStatus) {
		return nil
	}

	if !su.rateLimiter.Allow() {
		msg := fmt.Sprintf("Updating status is limited qps=%v burst=%v", statusUpdateQPS, statusUpdateBurst)
		klog.V(3).Infof(msg)
		return fmt.Errorf(msg)
	}

	klog.V(4).Infof("Updating status: %v", util.DumpJSON(newStatus))
	newNodeImage := imagePullNode.DeepCopy()
	newNodeImage.Status = newStatus

	_, err := su.imagePullNodeClient.UpdateStatus(context.TODO(), newNodeImage, metav1.UpdateOptions{})
	if err == nil {
		su.previousStatus = &newStatus
	}
	su.previousTimestamp = time.Now()
	return err
}

func (su *statusUpdater) statusChanaged(newStatus *appsv1alpha1.NodeImageStatus) bool {
	// 需要用previousStatus来对比，而不能用imagePullNode.Status，因为时间精度不同
	return !reflect.DeepEqual(su.previousStatus, newStatus)
}

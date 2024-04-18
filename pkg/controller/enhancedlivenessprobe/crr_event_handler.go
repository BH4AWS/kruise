package enhancedlivenessprobe

import (
	"reflect"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appsv1alpha1 "github.com/openkruise/kruise/apis/apps/v1alpha1"
)

var _ handler.EventHandler = &enqueueRequestForCRR{}

type enqueueRequestForCRR struct {
}

func (p *enqueueRequestForCRR) Create(evt event.CreateEvent, q workqueue.RateLimitingInterface) {
	obj, ok := evt.Object.(*appsv1alpha1.ContainerRecreateRequest)
	if !ok {
		return
	}
	p.enqueueCRRToNodePodProbe(q, obj)
	return
}

func (p *enqueueRequestForCRR) Delete(evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
	obj, ok := evt.Object.(*appsv1alpha1.ContainerRecreateRequest)
	if !ok {
		return
	}
	p.enqueueCRRToNodePodProbe(q, obj)
	return
}

func (p *enqueueRequestForCRR) Generic(evt event.GenericEvent, q workqueue.RateLimitingInterface) {
	obj, ok := evt.Object.(*appsv1alpha1.ContainerRecreateRequest)
	if !ok {
		return
	}
	p.enqueueCRRToNodePodProbe(q, obj)
	return
}

func (p *enqueueRequestForCRR) Update(evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
	new, ok := evt.ObjectNew.(*appsv1alpha1.ContainerRecreateRequest)
	if !ok {
		return
	}
	old, ok := evt.ObjectOld.(*appsv1alpha1.ContainerRecreateRequest)
	if !ok {
		return
	}
	if !reflect.DeepEqual(new.Status, old.Status) {
		p.enqueueCRRToNodePodProbe(q, new)
	}
	return
}

func (p *enqueueRequestForCRR) enqueueCRRToNodePodProbe(q workqueue.RateLimitingInterface, crr *appsv1alpha1.ContainerRecreateRequest) {
	if crr.Labels["usage"] == "" {
		return
	}
	q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
		Name: crr.Labels["usage"],
	}})
}

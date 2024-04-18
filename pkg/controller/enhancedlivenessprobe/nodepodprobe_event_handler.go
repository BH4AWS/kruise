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

var _ handler.EventHandler = &enqueueRequestForNodePodProbe{}

type enqueueRequestForNodePodProbe struct {
}

func (p *enqueueRequestForNodePodProbe) Create(evt event.CreateEvent, q workqueue.RateLimitingInterface) {
	obj, ok := evt.Object.(*appsv1alpha1.NodePodProbe)
	if !ok {
		return
	}
	p.enqueueNodePodProbe(q, obj)
	return
}

// when a nodePodProbe is deleted, all of CRRs related with this will be deleted by Cascading.
func (p *enqueueRequestForNodePodProbe) Delete(evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
	return
}

func (p *enqueueRequestForNodePodProbe) Generic(evt event.GenericEvent, q workqueue.RateLimitingInterface) {
	return
}

func (p *enqueueRequestForNodePodProbe) Update(evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
	// must be deep copy before update the objection
	new, ok := evt.ObjectNew.(*appsv1alpha1.NodePodProbe)
	if !ok {
		return
	}
	old, ok := evt.ObjectOld.(*appsv1alpha1.NodePodProbe)
	if !ok {
		return
	}
	if !reflect.DeepEqual(new.Status, old.Status) {
		p.enqueueNodePodProbe(q, new)
	}
	return
}

func (p *enqueueRequestForNodePodProbe) enqueueNodePodProbe(q workqueue.RateLimitingInterface, npp *appsv1alpha1.NodePodProbe) {
	q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
		Name: npp.GetName(),
	}})
}

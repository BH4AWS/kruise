package imagepullsecretscollection

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	concurrentReconciles = 3
)

// ReconcileImagePullSecretsCollection reconciles a ImagePullSecretsCollection object
type ReconcileImagePullSecretsCollection struct {
	client.Client
	scheme   *runtime.Scheme
	recorder record.EventRecorder
}

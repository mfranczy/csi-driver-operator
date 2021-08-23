package tenant

import (
	"context"

	csiprovisionerv1alpha1 "github.com/mfranczy/csi-driver-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const csiConfigMapName = "driver-config"

func getDesiredConfigMap(obj metav1.Object, infraClusterNamespace, infraClusterLabels string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      csiConfigMapName,
			Namespace: namespaceName,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(obj, csiprovisionerv1alpha1.GroupVersion.WithKind("Tenant")),
			},
		},
		Data: map[string]string{
			"infraClusterNamespace": infraClusterNamespace,
			"infraClusterLabels":    infraClusterLabels,
		},
	}
}

func (r *TenantReconciler) reconcileConfigMap(ctx context.Context, obj metav1.Object, infraClusterNamespace,
	infraClusterLabels string) (controllerutil.OperationResult, error) {
	l := log.FromContext(ctx).WithName("config-map")
	l.Info("Reconciling config map", "name", csiConfigMapName)

	desiredConfigMapObj := getDesiredConfigMap(obj, infraClusterNamespace, infraClusterLabels)
	currentConfigMapObj := desiredConfigMapObj.DeepCopyObject().(*corev1.ConfigMap)
	return ctrl.CreateOrUpdate(ctx, r.Client, currentConfigMapObj, func() error {
		currentConfigMapObj.OwnerReferences = desiredConfigMapObj.OwnerReferences
		currentConfigMapObj.Data = desiredConfigMapObj.Data
		return nil
	})
}

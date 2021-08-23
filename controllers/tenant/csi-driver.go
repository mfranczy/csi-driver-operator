package tenant

import (
	"context"

	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	csiprovisionerv1alpha1 "github.com/mfranczy/csi-driver-operator/api/v1alpha1"
)

const csiDriverName = "csi.kubevirt.io"

func getDesiredCSIDriverObj(obj metav1.Object) *storagev1.CSIDriver {
	return &storagev1.CSIDriver{
		ObjectMeta: metav1.ObjectMeta{
			Name: csiDriverName,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(obj, csiprovisionerv1alpha1.GroupVersion.WithKind("Tenant")),
			},
		},
		Spec: storagev1.CSIDriverSpec{
			AttachRequired: pointer.Bool(true),
			PodInfoOnMount: pointer.Bool(true),
		},
	}
}

func (r *TenantReconciler) reconcileCSIDriver(ctx context.Context, obj metav1.Object) (controllerutil.OperationResult, error) {
	l := log.FromContext(ctx).WithName("csi-driver")
	l.Info("Reconciling csi driver", "name", csiDriverName)

	desiredCSIObj := getDesiredCSIDriverObj(obj)
	currentCSIObj := desiredCSIObj.DeepCopyObject().(*storagev1.CSIDriver)
	return ctrl.CreateOrUpdate(ctx, r.Client, currentCSIObj, func() error {
		currentCSIObj.OwnerReferences = desiredCSIObj.OwnerReferences
		return nil
	})
}

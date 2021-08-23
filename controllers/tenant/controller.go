/*
Copyright 2021.

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

package tenant

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	csiprovisionerv1alpha1 "github.com/mfranczy/csi-driver-operator/api/v1alpha1"
)

const (
	namespaceName          = "kubevirt-csi-driver"
	tenantName             = "tenant"
	infraClusterSecretName = "infra-cluster-credentials"
)

// TenantReconciler reconciles a Tenant object
type TenantReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=csiprovisioner.kubevirt.io,resources=tenants,verbs=get;list;watch
//+kubebuilder:rbac:groups=csiprovisioner.kubevirt.io,resources=tenants/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=csiprovisioner.kubevirt.io,resources=tenants/finalizers,verbs=update

//+kubebuilder:rbac:groups="",resources=persistentvolumes,verbs="*"
//+kubebuilder:rbac:groups="",resources=nodes;persistentvolumeclaims;persistentvolumeclaims/status,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups="",resources=serviceaccounts;configmaps;secrets;events,verbs=get;list;watch;update;patch;create
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles;clusterrolebindings,verbs=get;list;watch;update;patch;create
//+kubebuilder:rbac:groups=extensions;apps,resources=deployments;daemonsets,verbs=get;list;watch;update;patch;create
//+kubebuilder:rbac:groups=storage.k8s.io,resources=storageclasses;csidrivers;volumeattachments;volumeattachments/status,verbs=get;list;watch;update;patch;create
//+kubebuilder:rbac:groups=storage.k8s.io;csi.storage.k8s.io,resources=csinodes;csinodeinfos,verbs=get;list;watch
//+kubebuilder:rbac:groups=storage.k8s.io;csi.storage.k8s.io,resources=csidrivers,verbs=get;list;watch;update;create
//+kubebuilder:rbac:groups=snapshot.storage.k8s.io,resources=volumesnapshots;volumesnapshots/status,verbs=get;list;watch;update
//+kubebuilder:rbac:groups=snapshot.storage.k8s.io,resources=volumesnapshotclasses,verbs=get;list;watch
//+kubebuilder:rbac:groups=snapshot.storage.k8s.io,resources=volumesnapshotcontents,verbs="*"
//+kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs="*"
//+kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=create;list

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *TenantReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx)

	tenant := csiprovisionerv1alpha1.Tenant{}
	err := r.Client.Get(ctx, req.NamespacedName, &tenant)
	if err != nil {
		if errors.IsNotFound(err) {
			l.Info("Tenant instance not found", "name", req.NamespacedName)
			return ctrl.Result{}, nil
		}
		l.Info("Error reading the request object, requeuing.")
		return ctrl.Result{}, err
	}
	objMeta := tenant.GetObjectMeta()

	// wait for infra-credentials
	infraCredentials := corev1.Secret{}
	err = r.Get(ctx, client.ObjectKey{Name: infraClusterSecretName, Namespace: namespaceName}, &infraCredentials)
	if err != nil {
		if errors.IsNotFound(err) {
			l.Info("Waiting for infra-cluster-credentials secret to be created", "name", req.NamespacedName)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	_, err = r.reconcileConfigMap(ctx, objMeta, tenant.Spec.InfraClusterNamespace, tenant.Spec.InfraClusterLabels)
	if err != nil {
		l.Info("Error reconciling csi driver, requeuing.")
		return ctrl.Result{}, err
	}

	_, err = r.reconcileCSIDriver(ctx, objMeta)
	if err != nil {
		l.Info("Error reconciling csi driver, requeuing.")
		return ctrl.Result{}, err
	}

	_, err = r.reconcileRBAC(ctx, objMeta)
	if err != nil {
		l.Info("Error reconciling rbac, requeuing.")
		return ctrl.Result{}, err
	}

	_, err = r.reconcileDaemonset(ctx, objMeta, "", "")
	if err != nil {
		l.Info("Error reconciling daemonset, requeuing.")
		return ctrl.Result{}, err
	}

	_, err = r.reconcileDeployment(ctx, objMeta, "", "")
	if err != nil {
		l.Info("Error reconciling deployment, requeuing.")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TenantReconciler) SetupWithManager(mgr ctrl.Manager) error {
	filterTenants := predicate.Funcs{
		CreateFunc: func(event event.CreateEvent) bool {
			return event.Object.GetName() == tenantName
		},
	}

	filterSecrets := predicate.Funcs{
		CreateFunc: func(event event.CreateEvent) bool {
			return event.Object.GetName() == infraClusterSecretName
		},
	}

	mapEvents := func(obj client.Object) []reconcile.Request {
		return []reconcile.Request{
			{
				NamespacedName: types.NamespacedName{
					Name: tenantName,
				},
			},
		}
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&csiprovisionerv1alpha1.Tenant{}, builder.WithPredicates(filterTenants)).
		Owns(&corev1.ConfigMap{}).
		Owns(&storagev1.CSIDriver{}).
		Owns(&appsv1.DaemonSet{}).
		Owns(&appsv1.Deployment{}).
		Owns(&storagev1.StorageClass{}).
		Watches(&source.Kind{Type: &corev1.Secret{}}, handler.EnqueueRequestsFromMapFunc(mapEvents),
			builder.WithPredicates(filterSecrets)).
		Complete(r)
}

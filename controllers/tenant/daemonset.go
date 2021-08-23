package tenant

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	csiprovisionerv1alpha1 "github.com/mfranczy/csi-driver-operator/api/v1alpha1"
)

const (
	csiDaemonSetName = "kubevirt-csi-node"
)

func getDesiredDaemonSet(obj metav1.Object, imageRepository, imageTag string) *appsv1.DaemonSet {
	mountPropagationBidirectional := corev1.MountPropagationBidirectional
	hostPathDirectory := corev1.HostPathDirectory
	hostPathDirectoryOrCreate := corev1.HostPathDirectoryOrCreate

	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      csiDaemonSetName,
			Namespace: namespaceName,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(obj, csiprovisionerv1alpha1.GroupVersion.WithKind("Tenant")),
			},
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "kubevirt-csi-driver",
				},
			},
			UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				Type: appsv1.RollingUpdateDaemonSetStrategyType,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "kubevirt-csi-driver",
					},
				},
				Spec: corev1.PodSpec{
					HostNetwork:        true,
					ServiceAccountName: csiDaemonSetName,
					PriorityClassName:  "system-node-critical",
					Tolerations: []corev1.Toleration{
						{
							Operator: corev1.TolerationOpExists,
						},
					},
					Containers: []corev1.Container{
						{
							Name: "csi-driver",
							SecurityContext: &corev1.SecurityContext{
								Privileged:               pointer.BoolPtr(true),
								AllowPrivilegeEscalation: pointer.BoolPtr(true),
							},
							ImagePullPolicy: corev1.PullAlways,
							// TODO: change to image repository and tag
							Image: "quay.io/kubevirt/csi-driver:latest",
							Args: []string{
								"--endpoint=unix:/csi/csi.sock",
								"--namespace=kubevirt-csi-driver",
								"--node-name=$(KUBE_NODE_NAME)",
								"--infra-cluster-namespace=$(INFRACLUSTER_NAMESPACE)",
								"--infra-cluster-kubeconfig=/var/run/secrets/infracluster/kubeconfig",
							},
							Env: []corev1.EnvVar{
								{
									Name: "KUBE_NODE_NAME",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "spec.nodeName",
										},
									},
								},
								{
									Name: "INFRACLUSTER_NAMESPACE",
									ValueFrom: &corev1.EnvVarSource{
										ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "driver-config",
											},
											Key: "infraClusterNamespace",
										},
									},
								},
								{
									Name: "INFRACLUSTER_LABELS",
									ValueFrom: &corev1.EnvVarSource{
										ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "driver-config",
											},
											Key: "infraClusterLabels",
										},
									},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "infracluster",
									MountPath: "/var/run/secrets/infracluster",
								},
								{
									Name:             "kubelet-dir",
									MountPath:        "/var/lib/kubelet",
									MountPropagation: &mountPropagationBidirectional,
								},
								{
									Name:      "plugin-dir",
									MountPath: "/csi",
								},
								{
									Name:      "device-dir",
									MountPath: "/dev",
								},
								{
									Name:      "udev",
									MountPath: "/run/udev",
								},
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          "healthz",
									ContainerPort: 10300,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							LivenessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/healthz",
										Port: intstr.IntOrString{Type: intstr.String, StrVal: "healthz"},
									},
								},
								InitialDelaySeconds: 10,
								TimeoutSeconds:      3,
								PeriodSeconds:       10,
								FailureThreshold:    5,
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("50Mi"),
								},
							},
						},
						{
							Name: "csi-node-driver-registrar",
							SecurityContext: &corev1.SecurityContext{
								Privileged: pointer.BoolPtr(true),
							},
							ImagePullPolicy: corev1.PullAlways,
							// TODO: change to image repository and tag
							Image: "quay.io/openshift/origin-csi-node-driver-registrar:latest",
							Args: []string{
								"--csi-address=$(ADDRESS)",
								"--kubelet-registration-path=$(DRIVER_REG_SOCK_PATH)",
								"--v=5",
							},
							Lifecycle: &corev1.Lifecycle{
								PreStop: &corev1.Handler{
									Exec: &corev1.ExecAction{
										Command: []string{
											"/bin/sh", "-c", "rm -rf /registration/csi.kubevirt.io-reg.sock /csi/csi.sock",
										},
									},
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "ADDRESS",
									Value: "/csi/csi.sock",
								},
								{
									Name:  "DRIVER_REG_SOCK_PATH",
									Value: "/var/lib/kubelet/plugins/csi.kubevirt.io/csi.sock",
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "plugin-dir",
									MountPath: "/csi",
								},
								{
									Name:      "registration-dir",
									MountPath: "/registration",
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("5m"),
									corev1.ResourceMemory: resource.MustParse("20Mi"),
								},
							},
						},
						{
							Name:            "csi-liveness-probe",
							ImagePullPolicy: corev1.PullAlways,
							// TODO: change to image repository and tag
							Image: "quay.io/openshift/origin-csi-livenessprobe:latest",
							Args: []string{
								"--csi-address=/csi/csi.sock",
								"--probe-timeout=3s",
								"--health-port=10300",
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "plugin-dir",
									MountPath: "/csi",
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("5m"),
									corev1.ResourceMemory: resource.MustParse("20Mi"),
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "infracluster",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: infraClusterSecretName,
								},
							},
						},
						{
							Name: "kubelet-dir",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/var/lib/kubelet",
									Type: &hostPathDirectory,
								},
							},
						},
						{
							Name: "plugin-dir",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/var/lib/kubelet/plugins/csi.kubevirt.io/",
									Type: &hostPathDirectoryOrCreate,
								},
							},
						},
						{
							Name: "registration-dir",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/var/lib/kubelet/plugins_registry/",
									Type: &hostPathDirectory,
								},
							},
						},
						{
							Name: "device-dir",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/dev",
									Type: &hostPathDirectory,
								},
							},
						},
						{
							Name: "udev",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/run/udev",
								},
							},
						},
					},
				},
			},
		},
	}
}

func (r *TenantReconciler) reconcileDaemonset(ctx context.Context, obj metav1.Object, imageRepository, imageTag string) (controllerutil.OperationResult, error) {
	l := log.FromContext(ctx).WithName("daemonset")
	l.Info("Reconciling daemonset", "name", csiDaemonSetName)

	desiredDaemonSetObj := getDesiredDaemonSet(obj, "", "")
	currentDaemonSetObj := desiredDaemonSetObj.DeepCopyObject().(*appsv1.DaemonSet)
	return ctrl.CreateOrUpdate(ctx, r.Client, currentDaemonSetObj, func() error {
		currentDaemonSetObj.OwnerReferences = desiredDaemonSetObj.OwnerReferences
		currentDaemonSetObj.Spec = desiredDaemonSetObj.Spec
		return nil
	})
}

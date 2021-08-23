package tenant

import (
	"context"

	csiprovisionerv1alpha1 "github.com/mfranczy/csi-driver-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const csiDeploymentName = "kubevirt-csi-controller"

func getDesiredDeployment(obj metav1.Object, imageRepository, imageTag string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      csiDeploymentName,
			Namespace: namespaceName,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(obj, csiprovisionerv1alpha1.GroupVersion.WithKind("Tenant")),
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Int32(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "kubevirt-csi-driver",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "kubevirt-csi-driver",
					},
				},
				Spec: corev1.PodSpec{
					HostNetwork:        true,
					ServiceAccountName: csiDeploymentName,
					PriorityClassName:  "system-cluster-critical",
					Tolerations: []corev1.Toleration{
						{
							Key:      "CriticalAddonsOnly",
							Operator: corev1.TolerationOpExists,
						},
						{
							Key:      "node-role.kubernetes.io/master",
							Operator: corev1.TolerationOpExists,
							Effect:   corev1.TaintEffectNoSchedule,
						},
					},
					Containers: []corev1.Container{
						{
							Name:            "csi-driver",
							ImagePullPolicy: corev1.PullAlways,
							// TODO: change to image repository and tag
							Image: "quay.io/kubevirt/csi-driver:latest",
							Args: []string{
								"--endpoint=$(CSI_ENDPOINT)",
								"--namespace=kubevirt-csi-driver",
								"--infra-cluster-namespace=$(INFRACLUSTER_NAMESPACE)",
								"--infra-cluster-kubeconfig=/var/run/secrets/infracluster/kubeconfig",
								"--infra-cluster-labels=$(INFRACLUSTER_LABELS)",
								"--v=5",
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          "healthz",
									ContainerPort: 10301,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "CSI_ENDPOINT",
									Value: "unix:///var/lib/csi/sockets/pluginproxy/csi.sock",
								},
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
									Name:      "socket-dir",
									MountPath: "/var/lib/csi/sockets/pluginproxy/",
								},
								{
									Name:      "infracluster",
									MountPath: "/var/run/secrets/infracluster",
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("50Mi"),
								},
							},
						},
						{
							Name:            "csi-provisioner",
							ImagePullPolicy: corev1.PullAlways,
							// TODO: change to image repository and tag
							Image: "quay.io/openshift/origin-csi-external-provisioner:latest",
							Args: []string{
								"--csi-address=$(ADDRESS)",
								"--default-fstype=ext4",
								"--v=5",
							},
							Env: []corev1.EnvVar{
								{
									Name:  "ADDRESS",
									Value: "/var/lib/csi/sockets/pluginproxy/csi.sock",
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "socket-dir",
									MountPath: "/var/lib/csi/sockets/pluginproxy/",
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("50Mi"),
								},
							},
						},
						{
							Name:            "csi-attacher",
							ImagePullPolicy: corev1.PullAlways,
							// TODO: change to image repository and tag
							Image: "quay.io/openshift/origin-csi-external-attacher:latest",
							Args: []string{
								"--csi-address=$(ADDRESS)",
								"--v=5",
							},
							Env: []corev1.EnvVar{
								{
									Name:  "ADDRESS",
									Value: "/var/lib/csi/sockets/pluginproxy/csi.sock",
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "socket-dir",
									MountPath: "/var/lib/csi/sockets/pluginproxy/",
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("50Mi"),
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
								"--health-port=10301",
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "socket-dir",
									MountPath: "/csi",
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("50Mi"),
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "socket-dir",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
						{
							Name: "infracluster",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: infraClusterSecretName,
								},
							},
						},
					},
				},
			},
		},
	}
}

func (r *TenantReconciler) reconcileDeployment(ctx context.Context, obj metav1.Object, imageRepository, imageTag string) (controllerutil.OperationResult, error) {
	l := log.FromContext(ctx).WithName("deployment")
	l.Info("Reconciling deployment", "name", csiDeploymentName)

	desiredDeploymentObj := getDesiredDeployment(obj, "", "")
	currentDeploymentObj := desiredDeploymentObj.DeepCopyObject().(*appsv1.Deployment)
	return ctrl.CreateOrUpdate(ctx, r.Client, currentDeploymentObj, func() error {
		currentDeploymentObj.OwnerReferences = desiredDeploymentObj.OwnerReferences
		currentDeploymentObj.Spec = desiredDeploymentObj.Spec
		return nil
	})
}

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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// StorageClass represents a storage class that should reference a KubeVirt storage class on infra cluster.
type StorageClass struct {
	// Name of the storage class to use on the infrastructure cluster.
	InfraStorageClassName string `json:"infraStorageClassName"`
	// The VM bus type, defaults to scsi.
	// +optional
	Bus string `json:"bus,omitempty"`
}

// TenantSpec defines the desired state of Tenant.
type TenantSpec struct {
	// Set infraClusterNamespace to the KubeVirt cluster namespace.
	InfraClusterNamespace string `json:"infraClusterNamespace"`
	// The driver creates resources in the infra cluster.
	// These resources are labeled with the values you supply in infraClusterLabels.
	InfraClusterLabels string `json:"infraClusterLabels"`

	// Image repository address
	ImageRepository string `json:"imageRepository,omitempty"`
	// Image tag that should be used for all csi driver components
	ImageTag string `json:"imageTag,omitempty"`
	// StorageClasses represents storage classes that the tenant operator should create.
	// +optional
	StorageClasses []StorageClass `json:"storageClasses,omitempty"`
}

// TenantStatus defines the observed state of Tenant.
type TenantStatus struct {
	// Conditions represents resource conditions that operator reconciles.
	// +optional
	// +patchMergeKey=resource
	// +patchStrategy=merge,retainKeys
	ResourceConditions []ResourceStatusCondition `json:"resourceConditions,omitempty"`
}

// ResourceStatusCondition contains details for the current condition.
type ResourceStatusCondition struct {
	// Resource represents a k8s resource that has been created/updated by the operator.
	Resource string `json:"resource"`
	// OperationResult is the action result of a CreateOrUpdate call.
	OperationResult controllerutil.OperationResult `json:"operationResult"`
	// Last time the condition transitioned from one status to another.
	// +optional
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
	// Unique, one-word, CamelCase reason for the condition's last transition.
	// +optional
	Reason string `json:"reason,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster

// Tenant is the Schema for the tenants API
type Tenant struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TenantSpec   `json:"spec,omitempty"`
	Status TenantStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// TenantList contains a list of Tenant
type TenantList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Tenant `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Tenant{}, &TenantList{})
}

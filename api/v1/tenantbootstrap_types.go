/*
Copyright 2022.

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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// RbacRule defines a set of k8s rbac rules for a role
type RbacRule struct {
	ApiGroups []string `json:"apiGroups"`
	Resources []string `json:"resources"`
	Verbs     []string `json:"verbs"`
}

type Subject struct {
	Kind   string `json:"kind"`
	Name   string `json:"name"`
	Create bool   `json:"create"`
}

type Rbac struct {
	RoleName string     `json:"roleName"`
	Subjects []Subject  `json:"subjects"`
	Rules    []RbacRule `json:"rules"`
}

// TenantBootstrapSpec defines the desired state of TenantBootstrap
type TenantBootstrapSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	CreateNamespace bool   `json:"createNamespace"`
	Namespace       string `json:"namespace"`
	Rbac            []Rbac `json:"rbac"`
}

// TenantBootstrapStatus defines the observed state of TenantBootstrap
type TenantBootstrapStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// TenantBootstrap is the Schema for the tenantbootstraps API
type TenantBootstrap struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TenantBootstrapSpec   `json:"spec,omitempty"`
	Status TenantBootstrapStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// TenantBootstrapList contains a list of TenantBootstrap
type TenantBootstrapList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TenantBootstrap `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TenantBootstrap{}, &TenantBootstrapList{})
}

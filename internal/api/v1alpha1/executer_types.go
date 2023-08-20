/*
Copyright 2023.

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
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ExecuterSpec defines the desired state of Executer
type ExecuterSpec struct {
	// Image is the name of the image to be used for executer
	// +kubebuilder:validation:Required
	Image string `json:"image,omitempty"`

	// Commands is the command to be run inside the container
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems:=1
	Commands []string `json:"commands,omitempty"`

	// Replication is the replicas for the executer
	// +kubebuilder:validation:Optional
	Replication int32 `json:"replication,omitempty"`

	// Port is the container-port to be exposed
	// +kubebuilder:validation:Optional
	Port int32 `json:"port,omitempty"`

	// Ingress to expose executer service
	// +kubebuilder:validation:Optional
	Ingress *Ingress `json:"ingress,omitempty"`
}

type Ingress struct {
	// +kubebuilder:validation:Optional
	Name string `json:"name,omitempty"`
}

type ResourceState string

const (
	ResourceStateUnknown  ResourceState = "Unknown"
	ResourceStateIdle     ResourceState = "Idle"
	ResourceStateCreating ResourceState = "Creating"
	ResourceStateCreated  ResourceState = "Created"
	ResourceStateUpdating ResourceState = "Updating"
)

// ExecuterStatus defines the observed state of Executer
type ExecuterStatus struct {
	DeploymentState ResourceState `json:"deploymentState,omitempty"`
	ServiceState    ResourceState `json:"serviceState,omitempty"`
	IngressState    ResourceState `json:"ingressState,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Executer is the Schema for the executers API
type Executer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ExecuterSpec   `json:"spec,omitempty"`
	Status ExecuterStatus `json:"status,omitempty"`
}

func (e *Executer) ShouldCreateService() bool {
	return e.Spec.Port != 0
}

func (e *Executer) ShouldCreateIngress() bool {
	return e.Spec.Ingress != nil
}

//+kubebuilder:object:root=true

// ExecuterList contains a list of Executer
type ExecuterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Executer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Executer{}, &ExecuterList{})
}

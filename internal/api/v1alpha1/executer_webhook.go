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
	"errors"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var executerlog = logf.Log.WithName("executer-resource")

func (r *Executer) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

var _ webhook.Defaulter = &Executer{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *Executer) Default() {
	executerlog.Info("default", "name", r.Name)

	if r.ShouldCreateIngress() && r.Spec.Ingress.Name == "" {
		r.Spec.Ingress.Name = r.Name
	}

	r.Status.DeploymentState = ResourceStateUnknown
	r.Status.ServiceState = ResourceStateUnknown
	r.Status.IngressState = ResourceStateUnknown
}

var _ webhook.Validator = &Executer{}

func (r *Executer) checkSize() error {
	if r.Spec.Replication < 0 || r.Spec.Replication > 9 {
		return errors.New("Invalid Size value")
	}
	return nil
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Executer) ValidateCreate() error {
	executerlog.Info("validate create", "name", r.Name)

	if err := r.checkSize(); err != nil {
		return err
	}

	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Executer) ValidateUpdate(old runtime.Object) error {
	executerlog.Info("validate update", "name", r.Name)

	if err := r.checkSize(); err != nil {
		return err
	}

	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Executer) ValidateDelete() error {
	executerlog.Info("validate delete", "name", r.Name)
	return nil
}

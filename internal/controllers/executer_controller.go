package controllers

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/go-cmp/cmp"
	appsv1alpha1 "github.com/mohammadne/caas-operator/internal/api/v1alpha1"
	"github.com/mohammadne/caas-operator/internal/cloudflare"
	"github.com/mohammadne/caas-operator/internal/config"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	intstr "k8s.io/apimachinery/pkg/util/intstr"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const executerFinalizer = "apps.mohammadne.me/finalizer"

// ExecuterReconciler reconciles an Executer object
type ExecuterReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	Config     *config.Config
	Cloudflare cloudflare.Cloudflare
	Logger     *zap.Logger
}

// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *ExecuterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Logger.Named("reconcile")

	// Fetch the Executer instance
	// The purpose is check if the Custom Resource for the Kind Executer
	// is applied on the cluster if not we return nil to stop the reconciliation
	executer := &appsv1alpha1.Executer{}
	if err := r.Get(ctx, req.NamespacedName, executer); err != nil {
		if apierrors.IsNotFound(err) {
			// If the custom resource is not found then, it usually means that it was deleted or not created
			// In this way, we will stop the reconciliation
			log.Info("Executer resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}

		// Error reading the object - requeue the request.
		log.Error("Failed to get Executer resource", zap.Error(err))
		return ctrl.Result{}, err
	}

	if executer.GetDeletionTimestamp() != nil {
		if !controllerutil.ContainsFinalizer(executer, executerFinalizer) {
			return ctrl.Result{}, nil
		}

		host := r.host(executer, req.Namespace)
		if err := r.Cloudflare.DeleteRecord(host); err != nil {
			log.Error("Failed to delete cloudflare record, Requeue the reconcile loop", zap.Error(err))
			return ctrl.Result{Requeue: true}, err
		}

		log.Info("Removing Finalizer for Executer after successfully perform the operations")
		if ok := controllerutil.RemoveFinalizer(executer, executerFinalizer); !ok {
			err := errors.New("Failed to add finalizer into the Executer")
			log.Error("Requeue the reconcile loop", zap.Error(err))
			return ctrl.Result{Requeue: true}, nil
		}

		if err := r.Update(ctx, executer); err != nil {
			log.Error("Failed to remove finalizer for Executer", zap.Error(err))
			return ctrl.Result{}, err
		}
	}

	// ---------------------------------------------------------> Reconcile dependant objects

	result, err := r.ReconcileDeployment(ctx, req, executer)
	if err != nil || !result.IsZero() {
		return result, err
	}

	// don't continue if we don't need service
	if !executer.ShouldCreateService() {
		return ctrl.Result{}, nil
	}

	result, err = r.ReconcileService(ctx, req, executer)
	if err != nil || !result.IsZero() {
		return result, err
	}

	// don't continue if we don't need ingress
	if !executer.ShouldCreateIngress() {
		return ctrl.Result{}, nil
	}

	// We add finalizer only if we need an ingress object
	if !controllerutil.ContainsFinalizer(executer, executerFinalizer) {
		log.Info("Adding Finalizer for Executer")
		if ok := controllerutil.AddFinalizer(executer, executerFinalizer); !ok {
			err := errors.New("Failed to add finalizer into the Executer")
			log.Error("Requeue the reconcile loop", zap.Error(err))
			return ctrl.Result{Requeue: true}, nil
		}

		host := r.host(executer, req.Namespace)
		if err := r.Cloudflare.CreateRecord(host, r.Config.LoadbalancerIP); err != nil {
			if err == cloudflare.RecordAlreadyExists {
				if err := r.Cloudflare.UpdateRecord(host, r.Config.LoadbalancerIP); err != nil {
					log.Error("Failed to update cloudflare record", zap.Error(err))
					return ctrl.Result{}, err
				}
			} else {
				log.Error("Failed to create cloudflare record", zap.Error(err))
				return ctrl.Result{}, err
			}
		}

		if err := r.Update(ctx, executer); err != nil {
			log.Error("Failed to update Executer to add finalizer", zap.Error(err))
			return ctrl.Result{}, err
		}

		return ctrl.Result{Requeue: true}, nil
	}

	result, err = r.ReconcileIngress(ctx, req, executer)
	if err != nil || !result.IsZero() {
		return result, err
	}

	return ctrl.Result{}, nil
}

func labels(executer *appsv1alpha1.Executer) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "Executer",
		"app.kubernetes.io/instance":   executer.Name,
		"app.kubernetes.io/part-of":    "caas-operator",
		"app.kubernetes.io/created-by": "controller-manager",
	}
}
func (r *ExecuterReconciler) ReconcileDeployment(ctx context.Context, req ctrl.Request, executer *appsv1alpha1.Executer) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// create desired deployment and add the ownerReference for it
	desiredDeployment := deploymentTemplate(executer)
	if err := ctrl.SetControllerReference(executer, desiredDeployment, r.Scheme); err != nil {
		log.Error(err, "Failed to set reference", "NamespacedName", req.NamespacedName.String())
		return ctrl.Result{}, err
	}

	// Check if the deployment already exists, if not create a new one
	foundDeployment := &appsv1.Deployment{}
	if err := r.Get(ctx, req.NamespacedName, foundDeployment); err != nil && apierrors.IsNotFound(err) {
		executer.Status.DeploymentState = appsv1alpha1.ResourceStateCreating
		if err := r.Status().Update(ctx, executer); err != nil {
			log.Error(err, "Failed to update deployment state", "NamespacedName", req.NamespacedName.String())
			return ctrl.Result{}, err
		}

		log.Info("Creating a new Deployment", "NamespacedName", req.NamespacedName.String())
		if err = r.Create(ctx, desiredDeployment); err != nil {
			log.Error(err, "Failed to create new Deployment", "NamespacedName", req.NamespacedName.String())
			return ctrl.Result{}, err
		}

		// We will requeue the reconciliation so that we can ensure the state and move forward for the next operations
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Deployment")
		return ctrl.Result{}, err
	}

	// Update existing deployment spec
	if *foundDeployment.Spec.Replicas != *desiredDeployment.Spec.Replicas {
		executer.Status.DeploymentState = appsv1alpha1.ResourceStateUpdating
		if err := r.Status().Update(ctx, executer); err != nil {
			log.Error(err, "Failed to update deployment state", "NamespacedName", req.NamespacedName.String())
			return ctrl.Result{}, err
		}

		log.Info("Updating executer's deployment replicas", "found", foundDeployment.Spec.Replicas, "desired", desiredDeployment.Spec.Replicas)
		foundDeployment.Spec.Replicas = desiredDeployment.Spec.Replicas
		if err := r.Update(ctx, foundDeployment); err != nil {
			if strings.Contains(err.Error(), genericregistry.OptimisticLockErrorMsg) {
				return reconcile.Result{RequeueAfter: time.Millisecond * 500}, nil
			}

			log.Error(err, "Failed to update Deployment")
			return ctrl.Result{}, err
		}
	}

	if executer.Status.DeploymentState != appsv1alpha1.ResourceStateCreated {
		executer.Status.DeploymentState = appsv1alpha1.ResourceStateCreated
		if err := r.Status().Update(ctx, executer); err != nil {
			log.Error(err, "Failed to update deployment state", "NamespacedName", req.NamespacedName.String())
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func deploymentTemplate(executer *appsv1alpha1.Executer) *appsv1.Deployment {
	if executer.Spec.Replication == 0 {
		executer.Spec.Replication = 1
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      executer.Name,
			Namespace: executer.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &executer.Spec.Replication,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels(executer),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels(executer),
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            executer.Name,
							Image:           executer.Spec.Image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command:         executer.Spec.Commands,
						},
					},
				},
			},
		},
	}

	if executer.ShouldCreateService() {
		deployment.Spec.Template.Spec.Containers[0].Ports = []corev1.ContainerPort{
			{
				ContainerPort: executer.Spec.Port,
				Protocol:      corev1.ProtocolTCP,
			},
		}
	}

	return deployment
}

func (r *ExecuterReconciler) ReconcileService(ctx context.Context, req ctrl.Request, executer *appsv1alpha1.Executer) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// create desired service and add the ownerReference for it
	desiredService := serviceTemplate(executer)
	if err := ctrl.SetControllerReference(executer, desiredService, r.Scheme); err != nil {
		log.Error(err, "Failed to set reference", "NamespacedName", req.NamespacedName.String())
		return ctrl.Result{}, err
	}

	// Check if the service already exists, if not create a new one
	foundService := &corev1.Service{}
	if err := r.Get(ctx, req.NamespacedName, foundService); err != nil && apierrors.IsNotFound(err) {
		executer.Status.ServiceState = appsv1alpha1.ResourceStateCreating
		if err := r.Status().Update(ctx, executer); err != nil {
			log.Error(err, "Failed to update service state", "NamespacedName", req.NamespacedName.String())
			return ctrl.Result{}, err
		}

		log.Info("Creating a new Service", "NamespacedName", req.NamespacedName.String())
		if err = r.Create(ctx, desiredService); err != nil {
			log.Error(err, "Failed to create new Service", "NamespacedName", req.NamespacedName.String())
			return ctrl.Result{}, err
		}

		// We will requeue the reconciliation so that we can ensure the state and move forward for the next operations
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Service")
		return ctrl.Result{}, err
	}

	// Update existing service spec
	if foundService.Spec.Type != desiredService.Spec.Type ||
		cmp.Diff(foundService.Spec.Selector, desiredService.Spec.Selector) == "" ||
		len(foundService.Spec.Ports) != 1 ||
		foundService.Spec.Ports[0] != desiredService.Spec.Ports[0] {
		executer.Status.ServiceState = appsv1alpha1.ResourceStateUpdating
		if err := r.Status().Update(ctx, executer); err != nil {
			log.Error(err, "Failed to update service state", "NamespacedName", req.NamespacedName.String())
			return ctrl.Result{}, err
		}

		log.Info("Updating executer's service")
		foundService.Spec.Type = desiredService.Spec.Type
		foundService.Spec.Selector = desiredService.Spec.Selector
		foundService.Spec.Ports = desiredService.Spec.Ports
		if err := r.Update(ctx, foundService); err != nil {
			if strings.Contains(err.Error(), genericregistry.OptimisticLockErrorMsg) {
				return reconcile.Result{RequeueAfter: time.Millisecond * 500}, nil
			}

			log.Error(err, "Failed to update Service")
			return ctrl.Result{}, err
		}
	}

	if executer.Status.ServiceState != appsv1alpha1.ResourceStateCreated {
		executer.Status.ServiceState = appsv1alpha1.ResourceStateCreated
		if err := r.Status().Update(ctx, executer); err != nil {
			log.Error(err, "Failed to update service state", "NamespacedName", req.NamespacedName.String())
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func serviceTemplate(executer *appsv1alpha1.Executer) *corev1.Service {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      executer.Name,
			Namespace: executer.Namespace,
			Labels:    labels(executer),
		},
		Spec: corev1.ServiceSpec{
			Selector: labels(executer),
			Type:     corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Protocol:   corev1.ProtocolTCP,
					Port:       executer.Spec.Port,
					TargetPort: intstr.FromInt(int(executer.Spec.Port)),
				},
			},
		},
	}

	return service
}

func (r *ExecuterReconciler) host(executer *appsv1alpha1.Executer, namespace string) string {
	return fmt.Sprintf("%s.%s.caas.%s", executer.Spec.Ingress.Name, namespace, r.Config.Domain)
}

func (r *ExecuterReconciler) ReconcileIngress(ctx context.Context, req ctrl.Request, executer *appsv1alpha1.Executer) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// create desired ingress and add the ownerReference for it
	host := r.host(executer, req.Namespace)
	desiredIngress := ingressTemplate(executer, host)
	if err := ctrl.SetControllerReference(executer, desiredIngress, r.Scheme); err != nil {
		log.Error(err, "Failed to set reference", "NamespacedName", req.NamespacedName.String())
		return ctrl.Result{}, err
	}

	// Check if the ingress already exists, if not create a new one
	foundIngress := &networkingv1.Ingress{}
	if err := r.Get(ctx, req.NamespacedName, foundIngress); err != nil && apierrors.IsNotFound(err) {
		executer.Status.IngressState = appsv1alpha1.ResourceStateCreating
		if err := r.Status().Update(ctx, executer); err != nil {
			log.Error(err, "Failed to update ingress state", "NamespacedName", req.NamespacedName.String())
			return ctrl.Result{}, err
		}

		log.Info("Creating a new Ingress", "NamespacedName", req.NamespacedName.String())
		if err = r.Create(ctx, desiredIngress); err != nil {
			log.Error(err, "Failed to create new Ingress", "NamespacedName", req.NamespacedName.String())
			return ctrl.Result{}, err
		}

		// We will requeue the reconciliation so that we can ensure the state and move forward for the next operations
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Ingress")
		return ctrl.Result{}, err
	}

	// Update existing ingress spec
	if foundIngress.Annotations["cert-manager.io/cluster-issuer"] != desiredIngress.Annotations["cert-manager.io/cluster-issuer"] ||
		cmp.Diff(foundIngress.Labels, desiredIngress.Labels) == "" ||
		foundIngress.Spec.IngressClassName != desiredIngress.Spec.IngressClassName ||
		len(foundIngress.Spec.Rules) != 1 ||
		foundIngress.Spec.Rules[0] != desiredIngress.Spec.Rules[0] ||
		len(foundIngress.Spec.TLS) != 1 ||
		foundIngress.Spec.TLS[0].SecretName != desiredIngress.Spec.TLS[0].SecretName ||
		len(foundIngress.Spec.TLS[0].Hosts) != 1 ||
		foundIngress.Spec.TLS[0].Hosts[0] != desiredIngress.Spec.TLS[0].Hosts[0] {
		executer.Status.IngressState = appsv1alpha1.ResourceStateUpdating
		if err := r.Status().Update(ctx, executer); err != nil {
			log.Error(err, "Failed to update ingress state", "NamespacedName", req.NamespacedName.String())
			return ctrl.Result{}, err
		}

		log.Info("Updating executer's ingress")
		foundIngress.Spec.IngressClassName = desiredIngress.Spec.IngressClassName
		foundIngress.Spec.Rules = desiredIngress.Spec.Rules
		foundIngress.Spec.TLS = desiredIngress.Spec.TLS
		if err := r.Update(ctx, foundIngress); err != nil {
			if strings.Contains(err.Error(), genericregistry.OptimisticLockErrorMsg) {
				return reconcile.Result{RequeueAfter: time.Millisecond * 500}, nil
			}

			log.Error(err, "Failed to update Ingress")
			return ctrl.Result{}, err
		}
	}

	if executer.Status.IngressState != appsv1alpha1.ResourceStateCreated {
		executer.Status.IngressState = appsv1alpha1.ResourceStateCreated
		if err := r.Status().Update(ctx, executer); err != nil {
			log.Error(err, "Failed to update ingress state", "NamespacedName", req.NamespacedName.String())
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func ingressTemplate(executer *appsv1alpha1.Executer, host string) *networkingv1.Ingress {
	className := "nginx"
	pathType := networkingv1.PathTypePrefix

	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      executer.Name,
			Namespace: executer.Namespace,
			Labels:    labels(executer),
			Annotations: map[string]string{
				"cert-manager.io/cluster-issuer": "letsencrypt-production",
			},
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: &className,
			Rules: []networkingv1.IngressRule{
				{
					Host: host,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: &pathType,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: executer.Name,
											Port: networkingv1.ServiceBackendPort{
												Number: executer.Spec.Port,
											},
										},
									},
								},
							},
						},
					},
				},
			},
			TLS: []networkingv1.IngressTLS{
				{
					SecretName: executer.Name + "-executer-tls-certificate",
					Hosts:      []string{host},
				},
			},
		},
	}

	return ingress
}

// SetupWithManager sets up the controller with the Manager.
func (r *ExecuterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1alpha1.Executer{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Complete(r)
}

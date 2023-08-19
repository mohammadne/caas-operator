package controllers

import (
	"context"
	"time"

	appsv1alpha1 "github.com/mohammadne/caas-operator/internal/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ExecuterController", func() {
	Context("testing simple echo executer", func() {
		var executer *appsv1alpha1.Executer
		namespacedName := types.NamespacedName{Name: "test-executer-1", Namespace: "default"}
		ctx := context.Background()

		BeforeEach(func() {
			executer = &appsv1alpha1.Executer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      namespacedName.Name,
					Namespace: namespacedName.Namespace,
				},
				Spec: appsv1alpha1.ExecuterSpec{
					Image:       "ubuntu:latest",
					Commands:    []string{"echo", "hello world"},
					Replication: 1,
				},
			}
		})

		It("create executer without any issue", func() {
			err := k8sClient.Create(ctx, executer)
			Expect(err).To(BeNil())
		})

		It("should create deployment", func() {
			createdDeploy := &appsv1.Deployment{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, namespacedName, createdDeploy)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())
		})

		It("verify replicas for deployment", func() {
			createdDeploy := &appsv1.Deployment{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, namespacedName, createdDeploy)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())
			Expect(createdDeploy.Spec.Replicas).To(Equal(&executer.Spec.Replication))
		})

		It("should update deployment, once executer size is changed", func() {
			Expect(k8sClient.Get(ctx, namespacedName, executer)).Should(Succeed())

			// update size to 3
			executer.Spec.Replication = 3
			Expect(k8sClient.Update(ctx, executer)).Should(Succeed())
			Eventually(func() bool {
				k8sClient.Get(ctx, namespacedName, executer)
				return executer.Spec.Replication == 3
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			createdDeploy := &appsv1.Deployment{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, namespacedName, createdDeploy)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())
			Expect(createdDeploy.Spec.Replicas).To(Equal(&executer.Spec.Replication))
		})

		It("should not create service", func() {
			createdService := &corev1.Service{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, namespacedName, createdService)
				return err != nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())
		})

		It("delete executer without any issue", func() {
			Expect(k8sClient.Get(ctx, namespacedName, executer)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, executer)).Should(Succeed())
		})
	})

	Context("testing executer with port", func() {
		var executer *appsv1alpha1.Executer
		namespacedName := types.NamespacedName{Name: "test-executer-2", Namespace: "default"}
		ctx := context.Background()

		BeforeEach(func() {
			executer = &appsv1alpha1.Executer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      namespacedName.Name,
					Namespace: namespacedName.Namespace,
				},
				Spec: appsv1alpha1.ExecuterSpec{
					Image:       "python:latest",
					Commands:    []string{"python", "-m", "http.server", "8080"},
					Port:        8080,
					Replication: 2,
				},
			}
		})

		It("create executer without any issue", func() {
			err := k8sClient.Create(ctx, executer)
			Expect(err).To(BeNil())
		})

		It("should create service", func() {
			createdService := &corev1.Service{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, namespacedName, createdService)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())
		})

		It("delete executer without any issue", func() {
			Expect(k8sClient.Get(ctx, namespacedName, executer)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, executer)).Should(Succeed())
		})
	})

	Context("testing executer with ingress", func() {
		var executer *appsv1alpha1.Executer
		namespacedName := types.NamespacedName{Name: "test-executer-3", Namespace: "default"}
		ctx := context.Background()

		BeforeEach(func() {
			executer = &appsv1alpha1.Executer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      namespacedName.Name,
					Namespace: namespacedName.Namespace,
				},
				Spec: appsv1alpha1.ExecuterSpec{
					Image:       "python:latest",
					Commands:    []string{"python", "-m", "http.server", "8080"},
					Port:        8080,
					Replication: 2,
					Ingress: &appsv1alpha1.Ingress{
						Name: "test-executer",
					},
				},
			}
		})

		It("create executer without any issue", func() {
			err := k8sClient.Create(ctx, executer)
			Expect(err).To(BeNil())
		})

		It("should create ingress", func() {
			createdIngress := &networkingv1.Ingress{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, namespacedName, createdIngress)
				return err == nil
			}, time.Second*30, time.Millisecond*250).Should(BeTrue())
		})

		It("delete executer without any issue", func() {
			Expect(k8sClient.Get(ctx, namespacedName, executer)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, executer)).Should(Succeed())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, namespacedName, executer)
				return err != nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())
		})
	})
})

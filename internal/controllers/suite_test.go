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

package controllers

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1alpha1 "github.com/mohammadne/caas-operator/internal/api/v1alpha1"
	"github.com/mohammadne/caas-operator/internal/config"
	"github.com/mohammadne/caas-operator/pkg/logger"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var kubeconfig *rest.Config
var k8sClient client.Client
var testEnv *envtest.Environment

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

type mockCloudfalre struct{}

func (c *mockCloudfalre) CreateRecord(subdomain, ip string) error {
	return nil
}

func (c *mockCloudfalre) UpdateRecord(subdomain, ip string) error {
	return nil
}

func (c *mockCloudfalre) DeleteRecord(subdomain string) error {
	return nil
}

var _ = BeforeSuite(func() {
	var err error

	// Load kubeconfig from the specified path
	kubeconfigPath := filepath.Join(homedir.HomeDir(), ".kube", "config")
	kubeconfig, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	Expect(err).NotTo(HaveOccurred())
	Expect(kubeconfig).NotTo(BeNil())

	UseExistingCluster := true
	// ./bin/controller-gen crd paths="./..." output:crd:artifacts:config=config/crd/bases
	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		Config:                kubeconfig,
		UseExistingCluster:    &UseExistingCluster,
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
		CRDInstallOptions: envtest.CRDInstallOptions{
			MaxTime: 30 * time.Second,
		},
	}

	// kubeconfig is defined in this file globally.
	kubeconfig, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())

	err = appsv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(kubeconfig, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	k8sManager, err := ctrl.NewManager(kubeconfig, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).ToNot(HaveOccurred())

	// load dependencies for
	cfg := config.Load(false)
	logger := logger.NewZap(cfg.Logger)

	err = (&ExecuterReconciler{
		Client:     k8sManager.GetClient(),
		Scheme:     k8sManager.GetScheme(),
		Config:     cfg,
		Cloudflare: &mockCloudfalre{},
		Logger:     logger,
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	//+kubebuilder:scaffold:scheme
	go func() {
		defer GinkgoRecover()
		err = k8sManager.Start(context.TODO())
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
	}()

})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})

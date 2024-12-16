package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	k8s "github.com/adevinta/go-k8s-toolkit"
	"github.com/adevinta/go-testutils-toolkit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
	"sigs.k8s.io/e2e-framework/support/kind"
	"sigs.k8s.io/e2e-framework/third_party/helm"
	"sigs.k8s.io/external-dns/endpoint"
)

var (
	testenv             env.Environment
	releaseName         = "k8s-traffic-controller"
	kindClusterName     = envconf.RandomName("traffic-controller", 16)
	controllerNamespace = envconf.RandomName("controller", 16)
	testNamespace       = envconf.RandomName("ingress", 16)
)

func deleteControllerDeployment(t *testing.T, k8sClient client.Client) {
	t.Helper()

	require.NoError(t, k8sClient.Delete(context.Background(), &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      releaseName + "-controller-manager",
			Namespace: controllerNamespace,
		},
	}))
}

func installControllerChart(ctx context.Context, t *testing.T, k8sClient client.Client, args ...string) {
	t.Helper()
	helmClient := helm.New(testenv.EnvConf().KubeconfigFile())

	require.NoError(t, helmClient.RunUpgrade(
		helm.WithName(releaseName),
		helm.WithNamespace(controllerNamespace),
		helm.WithChart("../../helm-chart/traffic-controller"),
		helm.WithArgs(
			"--install",
		),
		helm.WithArgs(args...),
	))
}

func installCRD(ctx context.Context, t *testing.T, k8sClient client.Client, url string) {
	t.Helper()

	resp, err := http.Get(url)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	decoder := yaml.NewYAMLOrJSONDecoder(resp.Body, 4096)

	crd := &apiextensionsv1.CustomResourceDefinition{}
	require.NoError(t, decoder.Decode(crd))

	require.NoError(t, k8sClient.Create(ctx, crd))
}

func startMain(t *testing.T, args ...string) {
	t.Helper()
	os.Args = args
	go func() {
		main()
	}()
}

func TestTrafficControllerController(t *testing.T) {
	testutils.IntegrationTest(t)

	t.Setenv("KUBECONFIG", testenv.EnvConf().KubeconfigFile())
	osArgs := os.Args
	originalContext := mainContext
	t.Cleanup(func() {
		mainContext = originalContext
		os.Args = osArgs
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mainContext = ctx

	cfg, err := k8s.NewClientConfigBuilder().WithKubeConfigPath(testenv.EnvConf().KubeconfigFile()).Build()
	require.NoError(t, err)

	require.NoError(t, err)
	k8sClient, err := client.New(cfg, client.Options{Scheme: scheme})
	require.NoError(t, err)

	installCRD(ctx, t, k8sClient, "https://raw.githubusercontent.com/kubernetes-sigs/external-dns/refs/heads/master/docs/contributing/crd-source/crd-manifest.yaml")
	installControllerChart(ctx, t, k8sClient, "--set", "devMode=true")
	deleteControllerDeployment(t, k8sClient)

	sa := &v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      releaseName,
			Namespace: controllerNamespace,
		},
	}
	assert.Eventually(t, func() bool {
		err := k8sClient.Get(ctx, client.ObjectKeyFromObject(sa), sa)
		return err == nil
	}, 30*time.Second, 100*time.Millisecond)

	startMain(
		t,
		"k8s-traffic-controller",
		"--as", fmt.Sprintf("system:serviceaccount:%s:%s", controllerNamespace, releaseName),
		"--binding-domain", "example.com",
		"--cluster-name", kindClusterName,
		"--backend-type", "fake",
		"--initial-weight", "100",
	)

	ing := newIngress(testNamespace, "my-ingress", "ingress-lb.provider.com")
	// Create seems to update the status of the object.
	// Get a deep copy to be able to inject the status used by the controllers
	require.NoError(t, k8sClient.Create(ctx, ing.DeepCopy()))
	require.NoError(t, k8sClient.Status().Update(ctx, ing))

	require.NoError(t, k8sClient.Create(ctx, newIngressBackendServiceEndpoints(testNamespace, "my-ingress")))

	dnsEndPoint := &endpoint.DNSEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-ingress",
			Namespace: testNamespace,
		},
	}
	require.Eventually(t, func() bool {
		err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dnsEndPoint), dnsEndPoint)
		if err != nil {
			return false
		}
		// We expect exactly 1 entries
		if len(dnsEndPoint.Spec.Endpoints) != 1 {
			return false
		}
		return true
	}, 30*time.Second, 100*time.Millisecond)

	assert.True(t, hasDNSEndpointTarget(dnsEndPoint, kindClusterName, "ingress-lb.provider.com"))
	for _, e := range dnsEndPoint.Spec.Endpoints {
		if e.SetIdentifier == kindClusterName {
			assert.Contains(t, e.ProviderSpecific, endpoint.ProviderSpecificProperty{Name: "aws/weight", Value: "100"})
		}
	}

}

func hasDNSEndpointTarget(dnsEndPoint *endpoint.DNSEndpoint, identifier, target string) bool {
	for _, e := range dnsEndPoint.Spec.Endpoints {
		if e.SetIdentifier != identifier {
			continue
		}
		for _, t := range e.Targets {
			if t == target {
				return true
			}
		}
	}
	return false
}

func newIngressControllerService(loadbalancerName string) *v1.Service {
	return &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-service",
			Namespace: "default",
			Labels: map[string]string{
				"app": "my-ingress-controller",
			},
		},
		Spec: v1.ServiceSpec{
			Selector: map[string]string{
				"app": "my-service",
			},
			Ports: []v1.ServicePort{
				{
					Name: "http",
					Port: 80,
				},
			},
		},
		Status: v1.ServiceStatus{
			LoadBalancer: v1.LoadBalancerStatus{
				Ingress: []v1.LoadBalancerIngress{
					{
						Hostname: loadbalancerName,
					},
				},
			},
		},
	}
}

func p[T any](v T) *T {
	return &v
}

func newIngressBackendServiceEndpoints(namespace, name string) *v1.Endpoints {
	return &v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app": "my-service",
			},
		},
		Subsets: []v1.EndpointSubset{
			{
				Addresses: []v1.EndpointAddress{
					{
						IP: "10.0.0.1",
					},
				},
			},
		},
	}
}

func newIngress(namespace, name, loadBalancerHostName string) *networkingv1.Ingress {
	pathTypePrefix := networkingv1.PathTypePrefix
	return &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: p("public"),
			Rules: []networkingv1.IngressRule{
				{
					Host: "example.com",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: &pathTypePrefix,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: name,
											Port: networkingv1.ServiceBackendPort{
												Name: "http",
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		Status: networkingv1.IngressStatus{
			LoadBalancer: networkingv1.IngressLoadBalancerStatus{
				Ingress: []networkingv1.IngressLoadBalancerIngress{
					{
						Hostname: loadBalancerHostName,
					},
				},
			},
		},
	}
}

func TestMain(m *testing.M) {
	if os.Getenv("RUN_INTEGRATION_TESTS") == "true" {
		testenv = env.New()
		// Use pre-defined environment funcs to create a kind cluster prior to test run
		testenv.Setup(
			envfuncs.CreateCluster(kind.NewCluster(kindClusterName), kindClusterName),
			envfuncs.CreateNamespace(controllerNamespace),
			envfuncs.CreateNamespace(testNamespace),
		)

		// Use pre-defined environment funcs to teardown kind cluster after tests
		testenv.Finish(
			envfuncs.DeleteNamespace(controllerNamespace),
			// envfuncs.DestroyCluster(kindClusterName),
		)

		// launch package tests
		os.Exit(testenv.Run(m))
	} else {
		os.Exit(m.Run())
	}
}

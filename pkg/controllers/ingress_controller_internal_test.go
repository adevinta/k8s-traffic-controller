package controllers

import (
	"context"
	"testing"

	"github.com/pborman/uuid"
	"github.com/adevinta/k8s-traffic-controller/pkg/trafficweight"

	logruslogr "github.com/adevinta/go-log-toolkit"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/external-dns/endpoint"
)

func TestMissingIngressDeletesDNSEndpoints(t *testing.T) {
	extendedScheme := NewScheme()
	k8sClient := fake.NewClientBuilder().WithScheme(extendedScheme).WithObjects(
		&endpoint.DNSEndpoint{ObjectMeta: metav1.ObjectMeta{Namespace: "namespace1", Name: "ingress-name"}},
		&endpoint.DNSEndpoint{ObjectMeta: metav1.ObjectMeta{Namespace: "namespace2", Name: "ingress-name"}},
	).Build()
	ep := &endpoint.DNSEndpoint{ObjectMeta: metav1.ObjectMeta{Namespace: "namespace1", Name: "ingress-name"}}
	key := client.ObjectKeyFromObject(ep)

	reconciler := IngressReconciler{
		Client: k8sClient,
		Log:    logruslogr.NewLogr(&logrus.Logger{}),
	}

	reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "namespace1", Name: "ingress-name"}})

	assert.True(t, apierrors.IsNotFound(k8sClient.Get(context.Background(), key, ep)))

	ep = &endpoint.DNSEndpoint{ObjectMeta: metav1.ObjectMeta{Namespace: "namespace2", Name: "ingress-name"}}
	key = client.ObjectKeyFromObject(ep)
	assert.NoError(t, k8sClient.Get(context.Background(), key, ep))
}

func TestFilterIngressRulesByHost(t *testing.T) {
	reconciler := IngressReconciler{
		BindingDomain: "foo.io",
	}
	filtered := reconciler.filterIngressRulesByHost([]netv1.IngressRule{
		{
			Host: "zero.foo.io",
			IngressRuleValue: netv1.IngressRuleValue{
				HTTP: &netv1.HTTPIngressRuleValue{
					Paths: []netv1.HTTPIngressPath{
						{
							Path: "/path1",
						},
					},
				},
			},
		},
		{
			Host: "hello.bar.com",
			IngressRuleValue: netv1.IngressRuleValue{
				HTTP: &netv1.HTTPIngressRuleValue{
					Paths: []netv1.HTTPIngressPath{
						{
							Path: "/path1",
						},
					},
				},
			},
		},
		{
			Host: "two.foo.io",
			IngressRuleValue: netv1.IngressRuleValue{
				HTTP: &netv1.HTTPIngressRuleValue{
					Paths: []netv1.HTTPIngressPath{
						{
							Path: "/path2",
						},
					},
				},
			},
		},
		{
			Host: "world.bar.io",
		},
		{
			Host: "three.foo.io",
		},
	})

	assert.Len(t, filtered, 3)
	assert.Contains(t, filtered, netv1.IngressRule{
		Host: "zero.foo.io",
		IngressRuleValue: netv1.IngressRuleValue{
			HTTP: &netv1.HTTPIngressRuleValue{
				Paths: []netv1.HTTPIngressPath{
					{
						Path: "/path1",
					},
				},
			},
		},
	})
	assert.Contains(t, filtered, netv1.IngressRule{
		Host: "two.foo.io",
		IngressRuleValue: netv1.IngressRuleValue{
			HTTP: &netv1.HTTPIngressRuleValue{
				Paths: []netv1.HTTPIngressPath{
					{
						Path: "/path2",
					},
				},
			},
		},
	})
	assert.Contains(t, filtered, netv1.IngressRule{
		Host: "three.foo.io",
	})
}

func TestGetTargetFromIngress(t *testing.T) {
	t.Run("with dev mode, devmode should be returned", func(t *testing.T) {
		reconciler := IngressReconciler{
			DevMode: true,
		}
		target, err := reconciler.getTargetFromIngress(netv1.Ingress{})
		assert.NoError(t, err)
		assert.Equal(t, "devmode", target)
	})
	t.Run("with a Hostname target, the host name should be returned", func(t *testing.T) {
		reconciler := IngressReconciler{}
		target, err := reconciler.getTargetFromIngress(netv1.Ingress{
			Status: netv1.IngressStatus{
				LoadBalancer: netv1.IngressLoadBalancerStatus{
					Ingress: []netv1.IngressLoadBalancerIngress{
						{
							Hostname: "hello.world",
						},
					},
				},
			},
		})
		assert.NoError(t, err)
		assert.Equal(t, "hello.world", target)
	})
	t.Run("no target, an error should be returned", func(t *testing.T) {
		reconciler := IngressReconciler{}
		target, err := reconciler.getTargetFromIngress(netv1.Ingress{})
		assert.Error(t, err)
		assert.Equal(t, "", target)
	})
}

func TestReconcileIngressShouldCreateDNSEndpointsWithCorrectWeight(t *testing.T) {
	extendedScheme := NewScheme()

	ingress := mockIngress()
	svcEndpoint := mockEndpoint(epWithName("test-app"))
	svcEndpointA := mockEndpoint(epWithName("test-app-a"))

	k8sClient := fake.NewClientBuilder().WithScheme(extendedScheme).WithObjects(ingress, svcEndpoint, svcEndpointA).Build()

	reconciler := IngressReconciler{
		Client: k8sClient,
		Log:    logruslogr.NewLogr(&logrus.Logger{}),
	}

	trafficweight.Store.DesiredWeight = 100
	trafficweight.Store.CurrentWeight = 100

	reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "cpr-dev", Name: "test-app"}})

	ep := &endpoint.DNSEndpoint{ObjectMeta: metav1.ObjectMeta{Namespace: "cpr-dev", Name: "test-app"}}

	key := client.ObjectKeyFromObject(ep)
	assert.NoError(t, k8sClient.Get(context.Background(), key, ep))
	assert.Equal(t, "100", ep.Spec.Endpoints[0].ProviderSpecific[0].Value)
}

func TestIngressWithMissingPodsHaveZeroWeight(t *testing.T) {
	extendedScheme := NewScheme()

	ingress := mockIngress()
	testAppEndpoint := mockEndpoint(epWithName("test-app"), epWithoutSubset())
	testAppAEndpoint := mockEndpoint(epWithName("test-app-a"), epWithoutSubset())

	k8sClient := fake.NewClientBuilder().WithScheme(extendedScheme).WithObjects(ingress, testAppAEndpoint, testAppEndpoint).Build()

	reconciler := IngressReconciler{
		Client: k8sClient,
		Log:    logruslogr.NewLogr(&logrus.Logger{}),
	}

	trafficweight.Store.DesiredWeight = 100
	trafficweight.Store.CurrentWeight = 100

	reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "cpr-dev", Name: "test-app"}})

	ep := &endpoint.DNSEndpoint{ObjectMeta: metav1.ObjectMeta{Namespace: "cpr-dev", Name: "test-app"}}

	key := client.ObjectKeyFromObject(ep)
	assert.NoError(t, k8sClient.Get(context.Background(), key, ep))
	assert.Equal(t, "0", ep.Spec.Endpoints[0].ProviderSpecific[0].Value)
}

func TestIngressWithPartialMissingPodsHaveZeroWeight(t *testing.T) {
	extendedScheme := NewScheme()

	ingress := mockIngress()
	testAppEndpoint := mockEndpoint(epWithName("test-app"))
	testAppAEndpoint := mockEndpoint(epWithName("test-app-a"), epWithoutSubset())

	k8sClient := fake.NewClientBuilder().WithScheme(extendedScheme).WithObjects(ingress, testAppAEndpoint, testAppEndpoint).Build()

	reconciler := IngressReconciler{
		Client: k8sClient,
		Log:    logruslogr.NewLogr(&logrus.Logger{}),
	}

	trafficweight.Store.DesiredWeight = 100
	trafficweight.Store.CurrentWeight = 100

	reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "cpr-dev", Name: "test-app"}})

	ep := &endpoint.DNSEndpoint{ObjectMeta: metav1.ObjectMeta{Namespace: "cpr-dev", Name: "test-app"}}

	key := client.ObjectKeyFromObject(ep)
	assert.NoError(t, k8sClient.Get(context.Background(), key, ep))
	assert.Equal(t, "0", ep.Spec.Endpoints[0].ProviderSpecific[0].Value)
}

func TestIngressWithMissingPodsFaultyEndpointsHaveZeroWeight(t *testing.T) {
	extendedScheme := NewScheme()

	ingress := mockIngress()
	testAppEndpoint := mockEndpoint(epWithName("test-app"), epWithoutSubsetAddress())
	testAppAEndpoint := mockEndpoint(epWithName("test-app-a"), epWithoutSubsetAddress())

	k8sClient := fake.NewClientBuilder().WithScheme(extendedScheme).WithObjects(ingress, testAppAEndpoint, testAppEndpoint).Build()

	reconciler := IngressReconciler{
		Client: k8sClient,
		Log:    logruslogr.NewLogr(&logrus.Logger{}),
	}

	trafficweight.Store.DesiredWeight = 100
	trafficweight.Store.CurrentWeight = 100

	reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "cpr-dev", Name: "test-app"}})

	ep := &endpoint.DNSEndpoint{ObjectMeta: metav1.ObjectMeta{Namespace: "cpr-dev", Name: "test-app"}}

	key := client.ObjectKeyFromObject(ep)
	assert.NoError(t, k8sClient.Get(context.Background(), key, ep))
	assert.Equal(t, "0", ep.Spec.Endpoints[0].ProviderSpecific[0].Value)
}

func mockIngress(mutators ...func(*netv1.Ingress)) *netv1.Ingress {
	ing := netv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-app",
			Namespace: "cpr-dev",
		},
		Status: netv1.IngressStatus{
			LoadBalancer: netv1.IngressLoadBalancerStatus{
				Ingress: []netv1.IngressLoadBalancerIngress{
					{
						Hostname: "bar-celona",
						IP:       "127.0.0.1",
					},
				},
			},
		},
	}
	controller := true
	ownerRef := metav1.OwnerReference{
		APIVersion: ing.APIVersion,
		Kind:       ing.Kind,
		Name:       ing.GetName(),
		UID:        ing.GetUID(),
		Controller: &controller,
	}
	ing.ObjectMeta.OwnerReferences = []metav1.OwnerReference{ownerRef}

	ingRules := []netv1.IngressRule{
		newRule(
			ruleWithHost("test-app.domain.tld"),
			ruleWithHTTPPaths(
				newHTTPIngressPath(
					pathWithPathRoute("/"),
					pathWithBackendServiceName("test-app"),
				),
				newHTTPIngressPath(
					pathWithPathRoute("/a"),
					pathWithBackendServiceName("test-app-a"),
				),
			),
		),
	}

	ing.Spec.Rules = ingRules
	for _, mutate := range mutators {
		mutate(&ing)
	}
	return &ing
}

func withObjectName[T client.Object](name string) func(T) {
	return func(object T) {
		object.SetName(name)
	}
}

func withObjectNamespace[T client.Object](namespace string) func(T) {
	return func(object T) {
		object.SetNamespace(namespace)
	}
}

func ingressWithRules(rules ...netv1.IngressRule) func(*netv1.Ingress) {
	return func(ing *netv1.Ingress) {
		ing.Spec.Rules = rules
	}
}

func newRule(mutators ...func(*netv1.IngressRule)) netv1.IngressRule {
	rule := netv1.IngressRule{}
	for _, mutator := range mutators {
		mutator(&rule)
	}
	return rule
}

func ruleWithHost(host string) func(*netv1.IngressRule) {
	return func(rule *netv1.IngressRule) {
		rule.Host = host
	}
}

func ruleWithHTTPPaths(paths ...netv1.HTTPIngressPath) func(*netv1.IngressRule) {
	return func(rule *netv1.IngressRule) {
		rule.IngressRuleValue.HTTP = &netv1.HTTPIngressRuleValue{
			Paths: paths,
		}
	}
}

func newHTTPIngressPath(mutators ...func(*netv1.HTTPIngressPath)) netv1.HTTPIngressPath {
	path := netv1.HTTPIngressPath{}
	for _, mutator := range mutators {
		mutator(&path)
	}
	return path
}

func pathWithPathRoute(path string) func(*netv1.HTTPIngressPath) {
	return func(ingressPath *netv1.HTTPIngressPath) {
		ingressPath.Path = path
	}
}

func pathWithBackendServiceName(serviceName string) func(*netv1.HTTPIngressPath) {
	return func(ingressPath *netv1.HTTPIngressPath) {
		ingressPath.Backend.Service = &netv1.IngressServiceBackend{
			Name: serviceName,
		}
	}
}

func mockEndpoint(mutators ...func(*v1.Endpoints)) *v1.Endpoints {
	// by default, the name itself should not matter
	ep := &v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      uuid.New(),
			Namespace: "cpr-dev",
		},
		Subsets: []v1.EndpointSubset{
			{
				Addresses: []v1.EndpointAddress{
					{
						IP: "10.1.1.1",
					},
					{
						IP: "10.1.1.2",
					},
				},
			},
		},
	}
	for _, mutate := range mutators {
		mutate(ep)
	}
	return ep
}

func epWithName(name string) func(*v1.Endpoints) {
	return func(ep *v1.Endpoints) {
		ep.Name = name
	}
}

func epWithoutSubset() func(*v1.Endpoints) {
	return func(ep *v1.Endpoints) {
		ep.Subsets = nil
	}
}

func epWithoutSubsetAddress() func(*v1.Endpoints) {
	return func(ep *v1.Endpoints) {
		for i := range ep.Subsets {
			ep.Subsets[i].Addresses = nil
		}
	}
}

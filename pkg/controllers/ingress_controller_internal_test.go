package controllers

import (
	"context"
	"testing"

	"github.com/adevinta/k8s-traffic-controller/pkg/trafficweight"

	logruslogr "github.com/adevinta/go-log-toolkit"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
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

func TestReconcileIngressShouldCreateDNSEndpointsWithCorrectWeight(t *testing.T) {
	extendedScheme := NewScheme()

	ingress := MockIngress()
	svcEndpoint := MockEndpoints(EndpointsWithName("test-app"))
	svcEndpointA := MockEndpoints(EndpointsWithName("test-app-a"))

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

	ingress := MockIngress()
	testAppEndpoint := MockEndpoints(EndpointsWithName("test-app"), EndpointsWithoutSubset())
	testAppAEndpoint := MockEndpoints(EndpointsWithName("test-app-a"), EndpointsWithoutSubset())

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

	ingress := MockIngress()
	testAppEndpoint := MockEndpoints(EndpointsWithName("test-app"))
	testAppAEndpoint := MockEndpoints(EndpointsWithName("test-app-a"), EndpointsWithoutSubset())

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

	ingress := MockIngress()
	testAppEndpoint := MockEndpoints(EndpointsWithName("test-app"), EndpointsWithoutSubsetAddress())
	testAppAEndpoint := MockEndpoints(EndpointsWithName("test-app-a"), EndpointsWithoutSubsetAddress())

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

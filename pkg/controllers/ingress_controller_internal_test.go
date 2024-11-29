package controllers

import (
	"context"
	"testing"

	"github.com/adevinta/k8s-traffic-controller/pkg/trafficweight"

	logruslogr "github.com/adevinta/go-log-toolkit"

	ingressv1beta1 "github.com/adevinta/k8s-traffic-controller/pkg/apis/ingress.adevinta.com/v1beta1"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	extendedScheme, err := NewScheme()
	require.NoError(t, err)
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
	extendedScheme, err := NewScheme()
	require.NoError(t, err)

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

	_, err = reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "cpr-dev", Name: "test-app"}})
	require.NoError(t, err)

	ep := &endpoint.DNSEndpoint{ObjectMeta: metav1.ObjectMeta{Namespace: "cpr-dev", Name: "test-app"}}

	key := client.ObjectKeyFromObject(ep)
	assert.NoError(t, k8sClient.Get(context.Background(), key, ep))
	require.Len(t, ep.Spec.Endpoints, 1)
	assert.Equal(t, "100", ep.Spec.Endpoints[0].ProviderSpecific[0].Value)
}

func TestIngressWithMissingPodsHaveZeroWeight(t *testing.T) {
	extendedScheme, err := NewScheme()
	require.NoError(t, err)

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
	extendedScheme, err := NewScheme()
	require.NoError(t, err)

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
	extendedScheme, err := NewScheme()
	require.NoError(t, err)

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

func TestListClusterIngressServiceWeightCRDs(t *testing.T) {
	extendedScheme, err := NewScheme()
	require.NoError(t, err)

	ingress := MockIngress(WithObjectNamespace[*netv1.Ingress]("test-namespace"), WithIngressClass("test-class"))

	ingressWeight1 := MockIngressWeight(WithObjectName[*ingressv1beta1.ClusterIngressServiceDNSWeight]("weight-1"), WithCRDIngressNamespaces("test-namespace"))
	ingressWeight2 := MockIngressWeight(WithObjectName[*ingressv1beta1.ClusterIngressServiceDNSWeight]("weight-2"), WithCRDIngressClasssesSelector("test-class"))
	ingressWeight3 := MockIngressWeight(WithObjectName[*ingressv1beta1.ClusterIngressServiceDNSWeight]("weight-3"), WithCRDIngressNamespaces("other-namespace"))

	k8sClient := fake.NewClientBuilder().WithScheme(extendedScheme).WithObjects(ingressWeight1, ingressWeight2, ingressWeight3).Build()

	r := IngressReconciler{
		Client: k8sClient,
		Log:    logruslogr.NewLogr(&logrus.Logger{}),
	}

	weights, err := r.listClusterIngressServiceDNSWeightsForIngress(context.Background(), ingress)
	require.NoError(t, err)

	assert.Len(t, weights, 2)

	assertClientHasObjectsObjects(t, k8sClient, ingressWeight1, ingressWeight2)
}

func TestMutateDNSEndpoint(t *testing.T) {
	extendedScheme, err := NewScheme()
	require.NoError(t, err)

	ingressObject := MockIngress(
		WithIngressClass("test-class"),
		IngressWithRules(
			NewRule(RuleWithHost("example.com")),
		),
		IngressWithLoadBalancerNames("bar-celona"),
	)
	ingressWeight := MockIngressWeight(
		WithCRDWeight(30),
		WithCRDRecordIdentifier("record-id"),
		WithCRDIngressClasssesSelector("test-class"),
		WithCRDServiceNamespace("test-namespace"),
		WithCRDServiceSelector(metav1.LabelSelector{
			MatchLabels: map[string]string{"test-key": "test-value"},
		},
		),
	)
	ingressService := MockService(
		WithObjectName[*v1.Service]("service-1"), WithObjectNamespace[*v1.Service]("test-namespace"),
		WithObjectLabels[*v1.Service](map[string]string{"test-key": "test-value"}),
		WithServiceLoadBalancerHostnames("service-1"),
	)

	k8sClient := fake.NewClientBuilder().WithScheme(extendedScheme).WithObjects(
		ingressObject,
		ingressWeight,
		ingressService,
	).Build()

	trafficweight.Store.DesiredWeight = 100

	r := IngressReconciler{
		Client:      k8sClient,
		ClusterName: "test-cluster",
		Log:         logruslogr.NewLogr(&logrus.Logger{}),
	}
	dnsEndpoint := &endpoint.DNSEndpoint{}

	err = r.mutateDNSEndpoint(context.Background(), dnsEndpoint, *ingressObject)
	require.NoError(t, err)

	require.Len(t, dnsEndpoint.Spec.Endpoints, 2)
	for _, e := range dnsEndpoint.Spec.Endpoints {
		assert.Equal(t, "example.com", e.DNSName)
	}

	assert.Equal(t, "100", dnsEndpoint.Spec.Endpoints[0].ProviderSpecific[0].Value)
	assert.Equal(t, "test-cluster", dnsEndpoint.Spec.Endpoints[0].SetIdentifier)

	assert.Equal(t, "30", dnsEndpoint.Spec.Endpoints[1].ProviderSpecific[0].Value)
	assert.Equal(t, "record-id", dnsEndpoint.Spec.Endpoints[1].SetIdentifier)

}

func TestAddCRDsTargetsToDNSEndpoint(t *testing.T) {
	extendedScheme, err := NewScheme()
	require.NoError(t, err)

	ingress := MockIngress(WithObjectNamespace[*netv1.Ingress]("test-namespace"), WithIngressClass("test-class"))

	ingressWeight := MockIngressWeight(
		WithCRDWeight(30),
		WithCRDIngressClasssesSelector("test-class"),
		WithCRDServiceNamespace("test-namespace"),
		WithCRDServiceSelector(metav1.LabelSelector{
			MatchLabels: map[string]string{"test-key": "test-value"},
		},
		),
	)

	t.Run("When no service is available", func(t *testing.T) {
		k8sClient := fake.NewClientBuilder().WithScheme(extendedScheme).WithObjects(
			ingressWeight,
			MockService(WithObjectName[*v1.Service]("service-1"), WithObjectNamespace[*v1.Service]("test-namespace")),
		).Build()
		r := IngressReconciler{
			Client: k8sClient,
			Log:    logruslogr.NewLogr(&logrus.Logger{}),
		}
		dnsEndpoint := &endpoint.DNSEndpoint{}
		assert.Error(t, r.addCRDsTargetsToEndpoint(context.Background(), dnsEndpoint, ingress, 100, []string{"example.com"}))
	})

	t.Run("When too many services are available", func(t *testing.T) {
		k8sClient := fake.NewClientBuilder().WithScheme(extendedScheme).WithObjects(
			ingressWeight,
			MockService(WithObjectName[*v1.Service]("service-1"), WithObjectNamespace[*v1.Service]("test-namespace"), WithObjectLabels[*v1.Service](map[string]string{"test-key": "test-value"})),
			MockService(WithObjectName[*v1.Service]("service-2"), WithObjectNamespace[*v1.Service]("test-namespace"), WithObjectLabels[*v1.Service](map[string]string{"test-key": "test-value"})),
		).Build()
		r := IngressReconciler{
			Client: k8sClient,
			Log:    logruslogr.NewLogr(&logrus.Logger{}),
		}
		dnsEndpoint := &endpoint.DNSEndpoint{}
		assert.Error(t, r.addCRDsTargetsToEndpoint(context.Background(), dnsEndpoint, ingress, 100, []string{"example.com"}))
	})

	t.Run("When a single service has no endpoint", func(t *testing.T) {
		k8sClient := fake.NewClientBuilder().WithScheme(extendedScheme).WithObjects(
			MockIngressWeight(FromIngressWeight(ingressWeight), WithCRDWeight(30)),
			MockService(
				WithObjectName[*v1.Service]("service-1"), WithObjectNamespace[*v1.Service]("test-namespace"),
				WithObjectLabels[*v1.Service](map[string]string{"test-key": "test-value"}),
				WithServiceLoadBalancerHostnames(),
			),
		).Build()
		r := IngressReconciler{
			Client: k8sClient,
			Log:    logruslogr.NewLogr(&logrus.Logger{}),
		}

		dnsEndpoint := &endpoint.DNSEndpoint{}
		assert.NoError(t, r.addCRDsTargetsToEndpoint(context.Background(), dnsEndpoint, ingress, 100, []string{"example.com", "www.example.com"}))

		require.Len(t, dnsEndpoint.Spec.Endpoints, 0)
	})

	t.Run("When a single service is available", func(t *testing.T) {
		k8sClient := fake.NewClientBuilder().WithScheme(extendedScheme).WithObjects(
			MockIngressWeight(FromIngressWeight(ingressWeight), WithCRDWeight(30)),
			MockService(
				WithObjectName[*v1.Service]("service-1"), WithObjectNamespace[*v1.Service]("test-namespace"),
				WithObjectLabels[*v1.Service](map[string]string{"test-key": "test-value"}),
				WithServiceLoadBalancerHostnames("bar-celona"),
			),
		).Build()
		r := IngressReconciler{
			Client: k8sClient,
			Log:    logruslogr.NewLogr(&logrus.Logger{}),
		}

		dnsEndpoint := &endpoint.DNSEndpoint{}
		assert.NoError(t, r.addCRDsTargetsToEndpoint(context.Background(), dnsEndpoint, ingress, 100, []string{"example.com", "www.example.com"}))

		require.Len(t, dnsEndpoint.Spec.Endpoints, 2)
		assert.Equal(t, "example.com", dnsEndpoint.Spec.Endpoints[0].DNSName)
		assert.Equal(t, "www.example.com", dnsEndpoint.Spec.Endpoints[1].DNSName)
		for _, e := range dnsEndpoint.Spec.Endpoints {
			assert.Equal(t, "30", e.ProviderSpecific[0].Value)
			assert.Equal(t, endpoint.Targets{"bar-celona"}, e.Targets)
			assert.Equal(t, WeightProperty, e.ProviderSpecific[0].Name)
		}
	})
}

func assertClientHasObjectsObjects(t *testing.T, k8sClient client.Client, objects ...client.Object) bool {
	t.Helper()
	for _, o := range objects {
		key := client.ObjectKeyFromObject(o)
		err := k8sClient.Get(context.Background(), key, o)
		if apierrors.IsNotFound(err) {
			t.Errorf("Object %s not found", key)
			return false
		}
		if !assert.NoError(t, err) {
			return false
		}
	}
	return true
}

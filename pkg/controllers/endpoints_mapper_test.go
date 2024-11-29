package controllers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestEndpointsMappingShouldIgnoreIngressWithNoHTTPRule(t *testing.T) {
	extendedScheme, err := NewScheme()
	require.NoError(t, err)

	ingress := MockIngress(IngressWithRules(NewRule()))

	testAppEndpoint := MockEndpoints(EndpointsWithName(ingress.GetName()), EndpointsWithoutSubset())

	k8sClient := fake.NewClientBuilder().WithScheme(extendedScheme).WithObjects(
		testAppEndpoint,
		ingress,
	).Build()

	mapper := endpointsMapper{
		Client: k8sClient,
	}
	requests := mapper.mapToIngressRequests(context.Background(), &v1.Endpoints{ObjectMeta: metav1.ObjectMeta{
		Namespace: ingress.GetNamespace(),
		Name:      "test-service",
	}})

	assert.Empty(t, requests)
}

func TestEndpointsMappingShouldTriggerIngressReconcile(t *testing.T) {
	extendedScheme, err := NewScheme()
	require.NoError(t, err)
	serviceName := "service-a"

	ingress1 := MockIngress(
		WithObjectName[*netv1.Ingress]("ingress-using-service-in-first-backend"),
		IngressWithRules(
			NewRule(RuleWithHTTPPaths(
				NewHTTPIngressPath(
					PathWithBackendServiceName(serviceName),
				),
			),
			),
		),
	)
	ingress2 := MockIngress(
		WithObjectName[*netv1.Ingress]("ingress-using-service-in-second-backend"),
		IngressWithRules(
			NewRule(),
			NewRule(RuleWithHTTPPaths(
				NewHTTPIngressPath(
					PathWithBackendServiceName("other-service"),
				),
				NewHTTPIngressPath(
					PathWithBackendServiceName(serviceName),
				),
			),
			),
		),
	)

	testAppEndpoint := MockEndpoints(EndpointsWithName(serviceName), EndpointsWithoutSubset())

	k8sClient := fake.NewClientBuilder().WithScheme(extendedScheme).WithObjects(
		testAppEndpoint,
		ingress1,
		ingress2,
	).Build()

	mapper := endpointsMapper{
		Client: k8sClient,
	}
	requests := mapper.mapToIngressRequests(context.Background(), testAppEndpoint)

	assert.ElementsMatch(t,
		[]reconcile.Request{
			{NamespacedName: types.NamespacedName{Namespace: ingress1.GetNamespace(), Name: ingress1.GetName()}},
			{NamespacedName: types.NamespacedName{Namespace: ingress2.GetNamespace(), Name: ingress2.GetName()}},
		},
		requests,
	)
}

func TestEndpointsMappingExcludesOtherNamespaces(t *testing.T) {
	extendedScheme, err := NewScheme()
	require.NoError(t, err)
	serviceName := "service-a"

	ingress1 := MockIngress(
		WithObjectNamespace[*netv1.Ingress]("namespace-a"),
		IngressWithRules(
			NewRule(RuleWithHTTPPaths(
				NewHTTPIngressPath(
					PathWithBackendServiceName(serviceName),
				),
			),
			),
		),
	)
	ingress2 := MockIngress(
		WithObjectNamespace[*netv1.Ingress]("namespace-b"),
		IngressWithRules(
			NewRule(RuleWithHTTPPaths(
				NewHTTPIngressPath(
					PathWithBackendServiceName(serviceName),
				),
			),
			),
		),
	)

	testAppEndpoint := MockEndpoints(WithObjectNamespace[*v1.Endpoints]("namespace-a"), EndpointsWithName(serviceName), EndpointsWithoutSubset())

	k8sClient := fake.NewClientBuilder().WithScheme(extendedScheme).WithObjects(
		testAppEndpoint,
		ingress1,
		ingress2,
	).Build()

	mapper := endpointsMapper{
		Client: k8sClient,
	}
	requests := mapper.mapToIngressRequests(context.Background(), testAppEndpoint)

	assert.ElementsMatch(t,
		[]reconcile.Request{
			{NamespacedName: types.NamespacedName{Namespace: ingress1.GetNamespace(), Name: ingress1.GetName()}},
		},
		requests,
	)
}

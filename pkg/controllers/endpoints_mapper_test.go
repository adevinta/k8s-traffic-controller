package controllers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestEndpointsMappingShouldIgnoreIngressWithNoHTTPRule(t *testing.T) {
	extendedScheme := NewScheme()

	ingress := mockIngress(ingressWithRules(newRule()))

	testAppEndpoint := mockEndpoint(epWithName(ingress.GetName()), epWithoutSubset())

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
	extendedScheme := NewScheme()
	serviceName := "service-a"

	ingress1 := mockIngress(
		withObjectName[*netv1.Ingress]("ingress-using-service-in-first-backend"),
		ingressWithRules(
			newRule(ruleWithHTTPPaths(
				newHTTPIngressPath(
					pathWithBackendServiceName(serviceName),
				),
			),
			),
		),
	)
	ingress2 := mockIngress(
		withObjectName[*netv1.Ingress]("ingress-using-service-in-second-backend"),
		ingressWithRules(
			newRule(),
			newRule(ruleWithHTTPPaths(
				newHTTPIngressPath(
					pathWithBackendServiceName("other-service"),
				),
				newHTTPIngressPath(
					pathWithBackendServiceName(serviceName),
				),
			),
			),
		),
	)

	testAppEndpoint := mockEndpoint(epWithName(serviceName), epWithoutSubset())

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
	extendedScheme := NewScheme()
	serviceName := "service-a"

	ingress1 := mockIngress(
		withObjectNamespace[*netv1.Ingress]("namespace-a"),
		ingressWithRules(
			newRule(ruleWithHTTPPaths(
				newHTTPIngressPath(
					pathWithBackendServiceName(serviceName),
				),
			),
			),
		),
	)
	ingress2 := mockIngress(
		withObjectNamespace[*netv1.Ingress]("namespace-b"),
		ingressWithRules(
			newRule(ruleWithHTTPPaths(
				newHTTPIngressPath(
					pathWithBackendServiceName(serviceName),
				),
			),
			),
		),
	)

	testAppEndpoint := mockEndpoint(withObjectNamespace[*v1.Endpoints]("namespace-a"), epWithName(serviceName), epWithoutSubset())

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

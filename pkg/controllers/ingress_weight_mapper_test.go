package controllers

import (
	"context"
	"testing"

	ingressv1beta1 "github.com/adevinta/k8s-traffic-controller/pkg/apis/ingress.adevinta.com/v1beta1"
	"github.com/stretchr/testify/assert"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestIngressMapper(t *testing.T) {

	ingress1 := MockIngress(WithIngressClass("nginx"), WithObjectName[*netv1.Ingress]("ingress-1"), WithObjectNamespace[*netv1.Ingress]("my-namespace"), WithObjectLabels[*netv1.Ingress](
		map[string]string{
			"app":   "my-app",
			"other": "label",
		},
	))

	ingress2 := MockIngress(FromIngress(ingress1), WithObjectName[*netv1.Ingress]("ingress-2"), WithObjectNamespace[*netv1.Ingress]("other-namespace"))
	ingress3 := MockIngress(FromIngress(ingress1), WithObjectName[*netv1.Ingress]("ingress-3"), WithObjectLabels[*netv1.Ingress](map[string]string{"app": "my-app"}))

	k8sClient := fake.NewClientBuilder().WithObjects(ingress1, ingress2, ingress3).Build()

	mapper := ingressWeightMapper{Client: k8sClient}

	assert.ElementsMatch(
		t,
		[]reconcile.Request{
			{NamespacedName: types.NamespacedName{Namespace: "my-namespace", Name: "ingress-1"}},
			{NamespacedName: types.NamespacedName{Namespace: "other-namespace", Name: "ingress-2"}},
			{NamespacedName: types.NamespacedName{Namespace: "my-namespace", Name: "ingress-3"}},
		},
		mapper.mapToIngressRequests(context.Background(), &ingressv1beta1.ClusterIngressServiceDNSWeight{
			Spec: ingressv1beta1.ClusterIngressServiceDNSWeightSpec{
				IngressSelector: ingressv1beta1.IngressSelector{
					Classes: []string{"nginx"},
				},
			},
		}),
	)

	assert.ElementsMatch(
		t,
		[]reconcile.Request{
			{NamespacedName: types.NamespacedName{Namespace: "my-namespace", Name: "ingress-1"}},
			{NamespacedName: types.NamespacedName{Namespace: "my-namespace", Name: "ingress-3"}},
		},
		mapper.mapToIngressRequests(context.Background(), &ingressv1beta1.ClusterIngressServiceDNSWeight{
			Spec: ingressv1beta1.ClusterIngressServiceDNSWeightSpec{
				IngressSelector: ingressv1beta1.IngressSelector{
					Classes:    []string{"nginx"},
					Namespaces: []string{"my-namespace"},
				},
			},
		}),
	)

}

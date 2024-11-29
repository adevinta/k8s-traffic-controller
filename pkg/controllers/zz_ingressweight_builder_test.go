package controllers

import (
	ingressv1beta1 "github.com/adevinta/k8s-traffic-controller/pkg/apis/ingress.adevinta.com/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func MockIngressWeight(mutators ...func(*ingressv1beta1.ClusterIngressServiceDNSWeight)) *ingressv1beta1.ClusterIngressServiceDNSWeight {
	r := &ingressv1beta1.ClusterIngressServiceDNSWeight{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-weight",
		},
		Spec: ingressv1beta1.ClusterIngressServiceDNSWeightSpec{
			Weight:     50,
			Identifier: "test-identifier",
			ServiceSelector: ingressv1beta1.ServiceSelector{
				Namespace: "test-namespace",
				LabelSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						"test-key": "test-value",
					},
				},
			},
			IngressSelector: ingressv1beta1.IngressSelector{
				Classes: []string{"test-class"},
			},
		},
	}
	for _, mutate := range mutators {
		mutate(r)
	}
	return r
}

func FromIngressWeight(original *ingressv1beta1.ClusterIngressServiceDNSWeight) func(*ingressv1beta1.ClusterIngressServiceDNSWeight) {
	return func(copy *ingressv1beta1.ClusterIngressServiceDNSWeight) {
		original.DeepCopyInto(copy)
	}
}

func WithCRDWeight(weight uint) func(*ingressv1beta1.ClusterIngressServiceDNSWeight) {
	return func(cisd *ingressv1beta1.ClusterIngressServiceDNSWeight) {
		cisd.Spec.Weight = weight
	}
}

func WithCRDIngressClasssesSelector(classes ...string) func(*ingressv1beta1.ClusterIngressServiceDNSWeight) {
	return func(cisd *ingressv1beta1.ClusterIngressServiceDNSWeight) {
		cisd.Spec.IngressSelector.Classes = classes
	}
}

func WithCRDIngresselector(selector metav1.LabelSelector) func(*ingressv1beta1.ClusterIngressServiceDNSWeight) {
	return func(cisd *ingressv1beta1.ClusterIngressServiceDNSWeight) {
		cisd.Spec.IngressSelector.LabelSelector = selector
	}
}

func WithCRDIngressNamespaces(namespaces ...string) func(*ingressv1beta1.ClusterIngressServiceDNSWeight) {
	return func(cisd *ingressv1beta1.ClusterIngressServiceDNSWeight) {
		cisd.Spec.IngressSelector.Namespaces = namespaces
	}
}

func WithCRDRecordIdentifier(identifier string) func(*ingressv1beta1.ClusterIngressServiceDNSWeight) {
	return func(cisd *ingressv1beta1.ClusterIngressServiceDNSWeight) {
		cisd.Spec.Identifier = identifier
	}
}

func WithCRDServiceSelector(selector metav1.LabelSelector) func(*ingressv1beta1.ClusterIngressServiceDNSWeight) {
	return func(cisd *ingressv1beta1.ClusterIngressServiceDNSWeight) {
		cisd.Spec.ServiceSelector.LabelSelector = selector
	}
}

func WithCRDServiceNamespace(namespace string) func(*ingressv1beta1.ClusterIngressServiceDNSWeight) {
	return func(cisd *ingressv1beta1.ClusterIngressServiceDNSWeight) {
		cisd.Spec.ServiceSelector.Namespace = namespace
	}
}

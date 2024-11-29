package controllers

import (
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func MockIngress(mutators ...func(*netv1.Ingress)) *netv1.Ingress {
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
		NewRule(
			RuleWithHost("test-app.domain.tld"),
			RuleWithHTTPPaths(
				NewHTTPIngressPath(
					PathWithPathRoute("/"),
					PathWithBackendServiceName("test-app"),
				),
				NewHTTPIngressPath(
					PathWithPathRoute("/a"),
					PathWithBackendServiceName("test-app-a"),
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

func IngressWithRules(rules ...netv1.IngressRule) func(*netv1.Ingress) {
	return func(ing *netv1.Ingress) {
		ing.Spec.Rules = rules
	}
}

func NewRule(mutators ...func(*netv1.IngressRule)) netv1.IngressRule {
	rule := netv1.IngressRule{}
	for _, mutator := range mutators {
		mutator(&rule)
	}
	return rule
}

func RuleWithHost(host string) func(*netv1.IngressRule) {
	return func(rule *netv1.IngressRule) {
		rule.Host = host
	}
}

func RuleWithHTTPPaths(paths ...netv1.HTTPIngressPath) func(*netv1.IngressRule) {
	return func(rule *netv1.IngressRule) {
		rule.IngressRuleValue.HTTP = &netv1.HTTPIngressRuleValue{
			Paths: paths,
		}
	}
}

func NewHTTPIngressPath(mutators ...func(*netv1.HTTPIngressPath)) netv1.HTTPIngressPath {
	path := netv1.HTTPIngressPath{}
	for _, mutator := range mutators {
		mutator(&path)
	}
	return path
}

func PathWithPathRoute(path string) func(*netv1.HTTPIngressPath) {
	return func(ingressPath *netv1.HTTPIngressPath) {
		ingressPath.Path = path
	}
}

func PathWithBackendServiceName(serviceName string) func(*netv1.HTTPIngressPath) {
	return func(ingressPath *netv1.HTTPIngressPath) {
		ingressPath.Backend.Service = &netv1.IngressServiceBackend{
			Name: serviceName,
		}
	}
}

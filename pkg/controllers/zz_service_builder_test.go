package controllers

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func MockService(mutators ...func(*v1.Service)) *v1.Service {
	svc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-app",
			Namespace: "test-namespace",
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				{
					Port: 80,
				},
			},
		},
		Status: v1.ServiceStatus{
			LoadBalancer: v1.LoadBalancerStatus{
				Ingress: []v1.LoadBalancerIngress{
					{
						Hostname: "bar-celona",
					},
				},
			},
		},
	}
	for _, mutate := range mutators {
		mutate(svc)
	}
	return svc
}

func WithServiceLoadBalancerHostnames(hostnames ...string) func(*v1.Service) {
	return func(svc *v1.Service) {
		svc.Status.LoadBalancer.Ingress = nil
		for _, hostname := range hostnames {
			svc.Status.LoadBalancer.Ingress = append(svc.Status.LoadBalancer.Ingress, v1.LoadBalancerIngress{
				Hostname: hostname,
			})
		}
	}
}

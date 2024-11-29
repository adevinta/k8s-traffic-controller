package controllers

import (
	"github.com/pborman/uuid"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func MockEndpoints(mutators ...func(*v1.Endpoints)) *v1.Endpoints {
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

func EndpointsWithName(name string) func(*v1.Endpoints) {
	return func(ep *v1.Endpoints) {
		ep.Name = name
	}
}

func EndpointsWithoutSubset() func(*v1.Endpoints) {
	return func(ep *v1.Endpoints) {
		ep.Subsets = nil
	}
}

func EndpointsWithSubsets(subsets ...v1.EndpointSubset) func(*v1.Endpoints) {
	return func(ep *v1.Endpoints) {
		ep.Subsets = subsets
	}
}

func EndpointsWithoutSubsetAddress() func(*v1.Endpoints) {
	return func(ep *v1.Endpoints) {
		for i := range ep.Subsets {
			ep.Subsets[i].Addresses = nil
		}
	}
}

func NewSubsetWithAddressIPs(ips ...string) v1.EndpointSubset {
	subset := v1.EndpointSubset{}
	for _, ip := range ips {
		subset.Addresses = append(subset.Addresses, v1.EndpointAddress{IP: ip})
	}
	return subset
}

package controllers

import (
	"context"

	"github.com/adevinta/go-log-toolkit"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// EndpointReconciler reconciles a Endpoint object
type endpointsMapper struct {
	client.Client
}

var _ handler.MapFunc = (&endpointsMapper{}).mapToIngressRequests

func (r *endpointsMapper) mapToIngressRequests(ctx context.Context, object client.Object) []reconcile.Request {

	var (
		ingresses netv1.IngressList
		reqs      []reconcile.Request
	)

	err := r.List(context.Background(), &ingresses, &client.ListOptions{Namespace: object.GetNamespace()})
	if err != nil {
		log.DefaultLogger.WithContext(ctx).WithError(err).Info("failed to list ingresses, won't trigger endpoint updates")
		return reqs
	}

	for _, ing := range ingresses.Items {
		for _, rule := range ing.Spec.Rules {
			//cover empty HTTP rule case
			if rule.HTTP != nil {
				for _, path := range rule.HTTP.Paths {
					if path.Backend.Service.Name == object.GetName() {
						reqs = append(reqs, reconcile.Request{
							NamespacedName: types.NamespacedName{
								Namespace: ing.GetNamespace(),
								Name:      ing.GetName(),
							},
						})
					}
				}
			}
		}
	}

	return reqs

}

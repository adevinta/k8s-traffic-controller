package controllers

import (
	"context"

	"github.com/adevinta/go-log-toolkit"
	ingressv1beta1 "github.com/adevinta/k8s-traffic-controller/pkg/apis/ingress.adevinta.com/v1beta1"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// ingressWeightMapper maps a ClusterIngressServiceDNSWeight to requests for all ingresses using it
type ingressWeightMapper struct {
	client.Client
}

var _ handler.MapFunc = (&endpointsMapper{}).mapToIngressRequests

func (r *ingressWeightMapper) mapToIngressRequests(ctx context.Context, object client.Object) []reconcile.Request {

	var (
		ingresses netv1.IngressList
		reqs      []reconcile.Request
	)

	clusterIngressServiceDNSWeight, ok := object.(*ingressv1beta1.ClusterIngressServiceDNSWeight)
	if !ok {
		log.DefaultLogger.WithContext(ctx).Error("object is not a ClusterIngressServiceDNSWeight")
		return reqs
	}

	selector, err := metav1.LabelSelectorAsSelector(&clusterIngressServiceDNSWeight.Spec.IngressSelector.LabelSelector)
	if err != nil {
		log.DefaultLogger.WithContext(ctx).WithError(err).Error("failed to parse selector")
		return reqs
	}
	err = r.List(context.Background(), &ingresses, client.MatchingLabelsSelector{Selector: selector})
	if err != nil {
		log.DefaultLogger.WithContext(ctx).WithError(err).Info("failed to list ingresses, won't trigger endpoint updates")
		return reqs
	}

	for _, ing := range ingresses.Items {
		// ingressMatchesSelector also considers namespaces and ingress classes
		if ingressMatchesSelector(&ing, &clusterIngressServiceDNSWeight.Spec.IngressSelector) {
			reqs = append(reqs, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: ing.GetNamespace(),
					Name:      ing.GetName(),
				},
			})
		}
	}

	return reqs
}

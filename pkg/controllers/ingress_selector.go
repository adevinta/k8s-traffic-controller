package controllers

import (
	"github.com/adevinta/k8s-traffic-controller/pkg/apis/ingress.adevinta.com/v1beta1"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func ingressMatchesSelectorNamespace(ingress *netv1.Ingress, selector *v1beta1.IngressSelector) bool {
	if len(selector.Namespaces) == 0 {
		return true
	}
	for _, namespace := range selector.Namespaces {
		if namespace == ingress.Namespace {
			return true
		}
	}
	return false
}

func ingressMatchesSelectorClass(ingress *netv1.Ingress, selector *v1beta1.IngressSelector) bool {
	for _, class := range selector.Classes {
		if ingress.Annotations["kubernetes.io/ingress.class"] == class {
			return true
		}
		if ingress.Spec.IngressClassName != nil && *ingress.Spec.IngressClassName == class {
			return true
		}
	}
	return false
}

func ingressMatchesSelectorLabelSelector(ingress *netv1.Ingress, selector *v1beta1.IngressSelector) bool {
	labelSelector, err := metav1.LabelSelectorAsSelector(&selector.LabelSelector)
	if err != nil {
		return false
	}
	if labelSelector.Matches(labels.Set(ingress.Labels)) {
		return true
	}
	return true
}

func ingressMatchesSelector(ingress *netv1.Ingress, selector *v1beta1.IngressSelector) bool {
	if !ingressMatchesSelectorNamespace(ingress, selector) {
		return false
	}
	if !ingressMatchesSelectorClass(ingress, selector) {
		return false
	}
	if !ingressMatchesSelectorLabelSelector(ingress, selector) {
		return false
	}
	return true
}

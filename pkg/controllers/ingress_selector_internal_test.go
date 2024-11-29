package controllers

import (
	"testing"

	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/adevinta/k8s-traffic-controller/pkg/apis/ingress.adevinta.com/v1beta1"
	"github.com/stretchr/testify/assert"
)

func TestIngressMatchesSelector(t *testing.T) {
	matchingIngress := &netv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ingress",
			Namespace: "test-namespace",
			Labels: map[string]string{
				"app": "test-app",
			},
		},
		Spec: netv1.IngressSpec{
			IngressClassName: p("public"),
		},
	}
	selector := &v1beta1.IngressSelector{
		LabelSelector: metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app": "test-app",
			},
		},
		Namespaces: []string{"test-namespace", "other-namespace"},
		Classes:    []string{"public", "other-class"},
	}

	assert.True(t, ingressMatchesSelector(matchingIngress, selector))

	t.Run("When the ingress selector has no namespace selector", func(t *testing.T) {
		selector := selector.DeepCopy()
		selector.Namespaces = nil
		assert.True(t, ingressMatchesSelector(matchingIngress, selector))
	})

	t.Run("When the ingress is in another matching namespace", func(t *testing.T) {
		ingress := matchingIngress.DeepCopy()
		ingress.Namespace = "other-namespace"
		assert.True(t, ingressMatchesSelector(ingress, selector))
	})
	t.Run("When the ingress is a non-matching namespace", func(t *testing.T) {
		ingress := matchingIngress.DeepCopy()
		ingress.Namespace = "non-matching-namespace"
		assert.False(t, ingressMatchesSelector(ingress, selector))
	})

	t.Run("When the class name is in ingress annotations", func(t *testing.T) {
		ingress := matchingIngress.DeepCopy()
		ingress.Annotations = map[string]string{
			"kubernetes.io/ingress.class": "public",
		}
		ingress.Spec.IngressClassName = nil
		assert.True(t, ingressMatchesSelector(ingress, selector))
	})

	t.Run("When the ingress has another matching class", func(t *testing.T) {
		ingress := matchingIngress.DeepCopy()
		ingress.Spec.IngressClassName = p("other-class")
		assert.True(t, ingressMatchesSelector(ingress, selector))
	})
	t.Run("When the ingress has a non-matching class", func(t *testing.T) {
		ingress := matchingIngress.DeepCopy()
		ingress.Spec.IngressClassName = p("non-matching-class")
		assert.False(t, ingressMatchesSelector(ingress, selector))
	})
	t.Run("When the ingress has a no class", func(t *testing.T) {
		ingress := matchingIngress.DeepCopy()
		ingress.Spec.IngressClassName = nil
		assert.False(t, ingressMatchesSelector(ingress, selector))
	})
}

package controllers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSetOwnerRef(t *testing.T) {
	obj := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-namespace",
		},
	}
	ingress := &netv1.Ingress{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Ingress",
			APIVersion: "networking.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ingress",
			Namespace: "test-namespace",
			UID:       "test-uid",
		},
	}
	setOwnerRef(&obj, ingress)

	require.Len(t, obj.GetOwnerReferences(), 1)
	assert.EqualValues(t, "Ingress", obj.GetOwnerReferences()[0].Kind)
	assert.EqualValues(t, "networking.k8s.io/v1", obj.GetOwnerReferences()[0].APIVersion)
	assert.EqualValues(t, "test-ingress", obj.GetOwnerReferences()[0].Name)
	assert.EqualValues(t, "test-uid", obj.GetOwnerReferences()[0].UID)
	require.NotNil(t, obj.GetOwnerReferences()[0].Controller)
	assert.True(t, *obj.GetOwnerReferences()[0].Controller)
}

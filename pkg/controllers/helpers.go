package controllers

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func p[T any](v T) *T {
	return &v
}

func setOwnerRef(ownee, owner client.Object) {
	ownee.SetOwnerReferences(
		[]metav1.OwnerReference{
			{
				APIVersion: owner.GetObjectKind().GroupVersionKind().GroupVersion().String(),
				Kind:       owner.GetObjectKind().GroupVersionKind().Kind,
				Name:       owner.GetName(),
				UID:        owner.GetUID(),
				Controller: p(true),
			},
		},
	)
}

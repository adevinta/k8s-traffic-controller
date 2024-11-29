package controllers

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func WithObjectName[T client.Object](name string) func(T) {
	return func(object T) {
		object.SetName(name)
	}
}

func WithObjectNamespace[T client.Object](namespace string) func(T) {
	return func(object T) {
		object.SetNamespace(namespace)
	}
}

func WithObjectFinalizers[T client.Object](finalizers ...string) func(T) {
	return func(object T) {
		object.SetFinalizers(finalizers)
	}
}

func WithObjectDeletionTimestamp[T client.Object](timestamp metav1.Time) func(T) {
	return func(object T) {
		object.SetDeletionTimestamp(&timestamp)
	}
}

func WithObjectLabels[T client.Object](labels map[string]string) func(T) {
	return func(object T) {
		object.SetLabels(labels)
	}
}

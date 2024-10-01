package controllers

import (
	apis "github.com/adevinta/k8s-traffic-controller/pkg/apis/externaldns.k8s.io/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

func NewScheme() *runtime.Scheme {

	scheme := runtime.NewScheme()

	_ = clientgoscheme.AddToScheme(scheme)

	_ = apis.AddToScheme(scheme)

	return scheme
}

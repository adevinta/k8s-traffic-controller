package controllers

import (
	apis "github.com/adevinta/k8s-traffic-controller/pkg/apis/externaldns.k8s.io/v1alpha1"
	ingressv1beta1 "github.com/adevinta/k8s-traffic-controller/pkg/apis/ingress.adevinta.com/v1beta1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

func NewScheme() (*runtime.Scheme, error) {

	scheme := runtime.NewScheme()

	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		return nil, err
	}

	if err := apis.AddToScheme(scheme); err != nil {
		return nil, err
	}

	if err := ingressv1beta1.AddToScheme(scheme); err != nil {
		return nil, err
	}

	if err := apiextensionsv1.AddToScheme(scheme); err != nil {
		return nil, err
	}

	return scheme, nil
}

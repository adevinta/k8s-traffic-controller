package v1beta1_test

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubectl/pkg/scheme"

	"github.com/adevinta/k8s-traffic-controller/pkg/apis/ingress.adevinta.com/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFullyFlegedDeserialization(t *testing.T) {
	serialized := `kind: ClusterIngressServiceDNSWeight
spec:
  weight: 80
  identifier: "public-nginx-classic-elb"
  serviceSelector:
    namespace: "service-namespace"
    matchLabels:
      "some": "value"
  ingressSelector:
    matchLabels:
      "key": "value"
    namespaces:
    - "my-namespace"
    classes:
    - "public"
`

	clusteringressweight := v1beta1.ClusterIngressServiceDNSWeight{}
	object, _, err := scheme.Codecs.UniversalDeserializer().Decode([]byte(serialized), nil, &clusteringressweight)
	require.NoError(t, err)
	assert.Equal(
		t,
		&v1beta1.ClusterIngressServiceDNSWeight{
			TypeMeta: metav1.TypeMeta{
				Kind: "ClusterIngressServiceDNSWeight",
			},
			Spec: v1beta1.ClusterIngressServiceDNSWeightSpec{
				Weight:     80,
				Identifier: "public-nginx-classic-elb",
				ServiceSelector: v1beta1.ServiceSelector{
					Namespace: "service-namespace",
					LabelSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"some": "value",
						},
					},
				},
				IngressSelector: v1beta1.IngressSelector{
					LabelSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"key": "value",
						},
					},
					Namespaces: []string{"my-namespace"},
					Classes:    []string{"public"},
				},
			},
		},
		object,
	)
}

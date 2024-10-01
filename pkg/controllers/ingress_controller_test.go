package controllers

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	k8s "github.com/adevinta/go-k8s-toolkit"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	externaldnsk8siov1alpha1 "sigs.k8s.io/external-dns/endpoint"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	logruslogr "github.com/adevinta/go-log-toolkit"

	"github.com/adevinta/go-testutils-toolkit"
	"github.com/adevinta/k8s-traffic-controller/pkg/trafficweight"
)

func TestIngressController(t *testing.T) {
	testutils.IntegrationTest(t)
	scheme := NewScheme()

	kubeconfig, err := k8s.NewClientConfigBuilder().WithKubeConfigPath(testEnvConf.KubeconfigFile()).Build()
	require.NoError(t, err)

	k8sClient, err := client.New(kubeconfig, client.Options{Scheme: scheme})
	require.NoError(t, err)

	reconciler := IngressReconciler{
		Client:           k8sClient,
		AWSRegion:        "eu-west-7",
		ClusterName:      "foolanito",
		BindingDomain:    "foo.io",
		Log:              logruslogr.NewLogr(&logrus.Logger{}),
		AnnotationPrefix: "dns.adevinta.com",
	}

	controller := true
	ing := netv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "bar",
			Namespace:   "foo",
			Annotations: map[string]string{"foo": "bar"},
		},
		Status: netv1.IngressStatus{
			LoadBalancer: netv1.IngressLoadBalancerStatus{
				Ingress: []netv1.IngressLoadBalancerIngress{
					{
						Hostname: "bar-celona",
						IP:       "127.0.0.1",
					},
				},
			},
		},
	}

	ownerRef := metav1.OwnerReference{
		APIVersion: ing.APIVersion,
		Kind:       ing.Kind,
		Name:       ing.GetName(),
		UID:        ing.GetUID(),
		Controller: &controller,
	}

	ing.ObjectMeta.OwnerReferences = []metav1.OwnerReference{ownerRef}

	ingRules := []netv1.IngressRule{
		{Host: "zero.foo.io"},
		{Host: "one.bar.io"},
		{Host: "two.foo.io"},
		{Host: "three.foo.io"},
	}

	ing.Spec.Rules = ingRules

	expected := externaldnsk8siov1alpha1.DNSEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bar",
			Namespace: "foo",
			OwnerReferences: []metav1.OwnerReference{
				ownerRef,
			},
		},
		Spec: externaldnsk8siov1alpha1.DNSEndpointSpec{
			Endpoints: []*externaldnsk8siov1alpha1.Endpoint{
				{
					DNSName:       "zero.foo.io",
					Targets:       externaldnsk8siov1alpha1.Targets{"bar-celona"},
					RecordType:    "CNAME",
					SetIdentifier: "foolanito",
					ProviderSpecific: externaldnsk8siov1alpha1.ProviderSpecific{
						{Name: "aws/weight", Value: "0"},
					},
				},
				{
					DNSName:       "two.foo.io",
					Targets:       externaldnsk8siov1alpha1.Targets{"bar-celona"},
					RecordType:    "CNAME",
					SetIdentifier: "foolanito",
					ProviderSpecific: externaldnsk8siov1alpha1.ProviderSpecific{
						{Name: "aws/weight", Value: "0"},
					},
				},
				{
					DNSName:       "three.foo.io",
					Targets:       externaldnsk8siov1alpha1.Targets{"bar-celona"},
					RecordType:    "CNAME",
					SetIdentifier: "foolanito",
					ProviderSpecific: externaldnsk8siov1alpha1.ProviderSpecific{
						{Name: "aws/weight", Value: "0"},
					},
				},
			},
		},
	}

	t.Run("Should forge DNS Endpoints", func(t *testing.T) {
		forged := &externaldnsk8siov1alpha1.DNSEndpoint{}
		reconciler.newDnsEndpoint(context.Background(), forged, "bar-celona", ing, ownerRef)
		assert.Equal(t, expected, *forged)
	})

	t.Run("Should create a DNSentry that does not exist during reconcile", func(t *testing.T) {
		reconciler.Client = fake.NewClientBuilder().WithScheme(scheme).WithObjects(&ing).Build()

		err := reconciler.reconcileDNSEntries(context.Background(), ing, ownerRef)
		assert.NoError(t, err)

		ep := externaldnsk8siov1alpha1.DNSEndpoint{}
		err = reconciler.Client.Get(context.Background(), client.ObjectKeyFromObject(&expected), &ep)
		assert.NoError(t, err)
		assert.Equal(t, expected.GetName(), ep.GetName())
		assert.Equal(t, expected.GetNamespace(), ep.GetNamespace())
		assert.Equal(t, expected.Spec, ep.Spec)
	})

	t.Run("Should update an entry that already exists during reconcile", func(t *testing.T) {
		oldEndpoint := expected.DeepCopy()
		oldEndpoint.Spec.Endpoints[0].DNSName = "randomChange"

		reconciler.Client = fake.NewClientBuilder().WithScheme(scheme).WithObjects(&ing, oldEndpoint).Build()
		err := reconciler.reconcileDNSEntries(context.Background(), ing, ownerRef)
		assert.NoError(t, err)

		ep := externaldnsk8siov1alpha1.DNSEndpoint{}
		err = reconciler.Client.Get(context.Background(), client.ObjectKeyFromObject(&expected), &ep)
		assert.NoError(t, err)
		assert.Equal(t, expected.GetName(), ep.GetName())
		assert.Equal(t, expected.GetNamespace(), ep.GetNamespace())
		assert.Equal(t, expected.Spec, ep.Spec)
	})

	t.Run("Should create a DNSentry that match the annotation filter", func(t *testing.T) {
		reconciler.Client = fake.NewClientBuilder().WithScheme(scheme).WithObjects(&ing).Build()
		reconciler.AnnotationFilter = NewAnnotationFilter("foo=bar")

		err := reconciler.reconcileDNSEntries(context.Background(), ing, ownerRef)
		assert.NoError(t, err)

		ep := externaldnsk8siov1alpha1.DNSEndpoint{}
		err = reconciler.Client.Get(context.Background(), client.ObjectKeyFromObject(&expected), &ep)
		assert.NoError(t, err)
		assert.Equal(t, expected.GetName(), ep.GetName())
		assert.Equal(t, expected.GetNamespace(), ep.GetNamespace())
		assert.Equal(t, expected.Spec, ep.Spec)
	})

	t.Run("Should not create a DNSentry that does not match the annotation filter", func(t *testing.T) {
		reconciler.Client = fake.NewClientBuilder().WithScheme(scheme).WithObjects(&ing).Build()
		reconciler.AnnotationFilter = NewAnnotationFilter("foo=notbar")

		err := reconciler.reconcileDNSEntries(context.Background(), ing, ownerRef)

		ep := externaldnsk8siov1alpha1.DNSEndpoint{}
		err = reconciler.Client.Get(context.Background(), client.ObjectKeyFromObject(&expected), &ep)
		assert.Error(t, err)
		assert.True(t, apierrors.IsNotFound(err))
	})

	t.Run("if we dont set --aws-health-check-id ingress shouldnt have health property", func(t *testing.T) {
		trafficweight.Store.AWSHealthCheckID = ""
		forged := &externaldnsk8siov1alpha1.DNSEndpoint{}
		reconciler.newDnsEndpoint(context.Background(), forged, "bar-celona", ing, ownerRef)
		assert.Equal(t, expected, *forged)
	})

	t.Run("if we set --aws-health-check-id, ingress should have health property", func(t *testing.T) {
		trafficweight.Store.AWSHealthCheckID = "one-healthcheck-id"
		oldRules := ing.Spec.Rules
		ing.Spec.Rules = []netv1.IngressRule{
			{Host: "healthyDomain.foo.io"},
		}
		expected := externaldnsk8siov1alpha1.DNSEndpoint{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "bar",
				Namespace: "foo",
				OwnerReferences: []metav1.OwnerReference{
					ownerRef,
				},
			},
			Spec: externaldnsk8siov1alpha1.DNSEndpointSpec{
				Endpoints: []*externaldnsk8siov1alpha1.Endpoint{
					{
						DNSName:       "healthyDomain.foo.io",
						Targets:       externaldnsk8siov1alpha1.Targets{"bar-celona"},
						RecordType:    "CNAME",
						SetIdentifier: "foolanito",
						ProviderSpecific: externaldnsk8siov1alpha1.ProviderSpecific{
							{Name: "aws/weight", Value: "0"},
							{Name: "aws/health-check-id", Value: "one-healthcheck-id"},
						},
					},
				},
			},
		}

		forged := &externaldnsk8siov1alpha1.DNSEndpoint{}
		reconciler.newDnsEndpoint(context.Background(), forged, "bar-celona", ing, ownerRef)
		assert.Equal(t, expected, *forged)
		ing.Spec.Rules = oldRules
	})

	t.Run("Should set the right weights", func(t *testing.T) {
		var weightTests = []struct {
			trafficWeight     int
			backendPercentage int
			expectedWeight    uint
		}{
			{1, 1, 1},       // traffic-weight = 1 backendPercentage = 1  -> weight = 1 // Ideally this case should be weight = 0.5, but route53 may not accept non integer weight
			{0, 1, 0},       // traffic-weight = 0 -> backendPercentage = 1 -> weight = 0
			{50, 50, 25},    // traffic-weight = 50 -> backendPercentage = 50 -> weight = 25
			{10, 50, 5},     // traffic-weight = 10 -> backendPercentage = 50 -> weight = 5
			{100, 50, 50},   // traffic-weight = 100 -> backendPercentage = 50 -> weight = 50
			{100, 100, 100}, // traffic-weight = 100 -> backendPercentage = 100 -> weight = 100
			{100, 0, 0},     // traffic-weight = 100 -> backendPercentage = 0 -> weight = 0
			{120, 10, 10},   // traffic-weight = 120 -> backendPercentage = 10 -> weight = 10  // Maximum valid traffic-weight should be 100, if bigger then 100 is assumed
			{100, 120, 100}, // traffic-weight = 100 -> backendPercentage = 120 -> weight = 100 // Maximum valid backendPercentage should be 100, if bigger then 100 is assumed
			{10, 120, 10},   // traffic-weight = 10 -> backendPercentage = 120 -> weight = 10 // Maximum valid backendPercentage should be 100, if bigger then 100 is assumed
		}

		for _, testValues := range weightTests {
			trafficweight.Store.DesiredWeight = testValues.backendPercentage
			ingressObject := netv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "ingress-calculated-weight-tests",
					Namespace:   "tests",
					Annotations: map[string]string{"dns.adevinta.com/traffic-weight": fmt.Sprintf("%d", testValues.trafficWeight)},
				},
			}

			calculated, err := reconciler.calculateIngressWeight(ingressObject)
			assert.Equal(t, testValues.expectedWeight, calculated)
			assert.NoError(t, err)
		}
	})

	t.Run("Should fail with negative weights", func(t *testing.T) {
		var weightTests = []struct {
			trafficWeight     int
			backendPercentage int
		}{
			{-1, 1},
			{0, -1},
			{-50, -50},
		}
		for _, testValues := range weightTests {
			trafficweight.Store.DesiredWeight = testValues.backendPercentage
			ingressObject := netv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "ingress-calculated-weight-tests",
					Namespace:   "tests",
					Annotations: map[string]string{"dns.adevinta.com/traffic-weight": fmt.Sprintf("%d", testValues.trafficWeight)},
				},
			}

			_, err := reconciler.calculateIngressWeight(ingressObject)
			assert.Error(t, err)
		}
	})

	t.Run("Should fail with a parseError if the annotation is not an integer", func(t *testing.T) {
		ingressObject := netv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "ingress-calculated-weight-tests",
				Namespace:   "tests",
				Annotations: map[string]string{"dns.adevinta.com/traffic-weight": "A"},
			},
		}

		_, err := reconciler.calculateIngressWeight(ingressObject)
		assert.Error(t, err)
	})

	t.Run("Should requeue the notification after 5 seconds", func(t *testing.T) {
		now := metav1.NewTime(time.Now())
		ep := expected.DeepCopy()
		ep.ObjectMeta.DeletionTimestamp = &now
		ep.ObjectMeta.Finalizers = []string{"finalizer"}
		reconciler.Client = fake.NewClientBuilder().WithScheme(scheme).WithObjects(&ing, ep).Build()
		result, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "bar", Namespace: "foo"}})
		assert.NoError(t, err)
		assert.True(t, result.Requeue)
	})

	t.Run("Should get existing ingress", func(t *testing.T) {
		name := ing.Name
		ns := ing.Namespace
		var newIng netv1.Ingress
		err := reconciler.Get(context.Background(), types.NamespacedName{Name: name, Namespace: ns}, &newIng)
		fmt.Println(err)
		assert.NoError(t, err)
		assert.NotNil(t, newIng)
		err = reconciler.Delete(context.Background(), &ing)
		assert.NoError(t, err)
		assert.Equal(t, ing.ObjectMeta, newIng.ObjectMeta)
		assert.Equal(t, ing.Spec, newIng.Spec)
		assert.Equal(t, ing.Status, newIng.Status)
	})
}

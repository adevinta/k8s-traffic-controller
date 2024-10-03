package controllers

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"

	"github.com/adevinta/k8s-traffic-controller/pkg/trafficweight"
	"github.com/go-logr/logr"
	netv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	externaldnsk8siov1alpha1 "sigs.k8s.io/external-dns/endpoint"
)

type annotationFilter struct {
	key   string
	value string
}

type IngressReconciler struct {
	client.Client
	Log              logr.Logger
	Scheme           *runtime.Scheme
	ClusterName      string
	BindingDomain    string
	AWSRegion        string
	AnnotationFilter annotationFilter
	DevMode          bool
	AnnotationPrefix string
}

func NewAnnotationFilter(filter string) annotationFilter {
	filterKeyValue := strings.Split(filter, "=")
	if len(filterKeyValue) == 2 {
		return annotationFilter{
			key:   filterKeyValue[0],
			value: filterKeyValue[1],
		}
	}

	return annotationFilter{}
}

func (r *IngressReconciler) annotationKey(key string) string {
	return fmt.Sprintf("%s/%s", r.AnnotationPrefix, key)
}

func (r *IngressReconciler) ingressHasAnnotationKeyValue(ingress netv1.Ingress, key, value string) bool {
	for k, v := range ingress.Annotations {
		if k == key && v == value {
			return true
		}
	}
	return false
}

func (r *IngressReconciler) ingressHasAnnotationKey(ingress netv1.Ingress, key string) bool {
	for k := range ingress.Annotations {
		if k == key {
			return true
		}
	}
	return false
}

func (r *IngressReconciler) ingressAnnotationMatchFilter(ingress netv1.Ingress) bool {
	if (annotationFilter{}) == r.AnnotationFilter {
		return true
	}
	return r.ingressHasAnnotationKeyValue(ingress, r.AnnotationFilter.key, r.AnnotationFilter.value)
}

func (r *IngressReconciler) filterIngressRulesByHost(rules []netv1.IngressRule) []netv1.IngressRule {
	rulesToBind := []netv1.IngressRule{}
	for _, rule := range rules {
		if strings.HasSuffix(rule.Host, r.BindingDomain) {
			rulesToBind = append(rulesToBind, rule)
		}
	}
	return rulesToBind
}

func (r *IngressReconciler) getTargetFromIngress(ingress netv1.Ingress) (string, error) {
	if r.DevMode {
		return "devmode", nil
	}
	if len(ingress.Status.LoadBalancer.Ingress) == 0 {
		return "", fmt.Errorf("Ingress has no status")
	}
	return ingress.Status.LoadBalancer.Ingress[0].Hostname, nil
}

func (r *IngressReconciler) isIngressWeighted(ingress netv1.Ingress) bool {
	return r.ingressHasAnnotationKey(ingress, r.annotationKey("traffic-weight"))
}

func (r *IngressReconciler) calculateIngressWeight(ingress netv1.Ingress) (uint, error) {
	backendPercentage := float64(trafficweight.Store.DesiredWeight)
	if backendPercentage < 0 {
		return 0, fmt.Errorf("Cannot handle negative backend weights")
	}
	if backendPercentage > 100 {
		backendPercentage = float64(100.0)
	}
	backendPercentage = float64(backendPercentage) / float64(100)
	userDesiredWeight, err := strconv.ParseFloat(ingress.Annotations[r.annotationKey("traffic-weight")], 64)
	if err != nil {
		return 0, fmt.Errorf("Cannot parse annotation %v with value '%v'", r.annotationKey("traffic-weight"), ingress.Annotations[r.annotationKey("traffic-weight")])
	}
	if userDesiredWeight < 0 {
		return 0, fmt.Errorf("Cannot handle negative traffic weights")
	}
	if userDesiredWeight > 100 {
		userDesiredWeight = float64(100.0)
	}
	calculatedWeight := backendPercentage * userDesiredWeight
	desiredWeight := uint(math.Ceil(calculatedWeight))
	return desiredWeight, err
}

func (r *IngressReconciler) newDnsEndpoint(ctx context.Context, dnsEndpoint *externaldnsk8siov1alpha1.DNSEndpoint, target string, ingress netv1.Ingress, owner metav1.OwnerReference) {
	var desiredWeight uint
	var err error
	var healthCheckProperty *externaldnsk8siov1alpha1.ProviderSpecificProperty
	dnsEndpoint.Name = ingress.ObjectMeta.Name
	dnsEndpoint.Namespace = ingress.ObjectMeta.Namespace
	dnsEndpoint.SetOwnerReferences([]metav1.OwnerReference{owner})
	desiredWeight = uint(trafficweight.Store.DesiredWeight)
	if trafficweight.Store.AWSHealthCheckID != "" {
		healthCheckProperty = &externaldnsk8siov1alpha1.ProviderSpecificProperty{
			Name:  "aws/health-check-id",
			Value: trafficweight.Store.AWSHealthCheckID,
		}
	}
	if r.isIngressWeighted(ingress) {
		desiredWeight, err = r.calculateIngressWeight(ingress)
		if err != nil {
			log := r.Log.WithValues("IngressName", ingress.ObjectMeta.Name).WithValues("IngressNamespace", ingress.ObjectMeta.Namespace)
			log.Error(err, "something went wrong calculating the weight, doing nothing")
			return
		}
		if len(dnsEndpoint.ObjectMeta.Annotations) == 0 {
			dnsEndpoint.ObjectMeta.Annotations = make(map[string]string)
		}
	}
	dnsEndpoint.Spec = externaldnsk8siov1alpha1.DNSEndpointSpec{Endpoints: []*externaldnsk8siov1alpha1.Endpoint{}}
	for _, rule := range r.filterIngressRulesByHost(ingress.Spec.Rules) {

		if !r.ingressRuleHasPods(ctx, ingress.ObjectMeta.Namespace, &rule) {
			desiredWeight = 0
		}

		providerSpecificProperties := externaldnsk8siov1alpha1.ProviderSpecific{
			externaldnsk8siov1alpha1.ProviderSpecificProperty{
				Name:  "aws/weight",
				Value: strconv.FormatUint(uint64(desiredWeight), 10),
			},
		}
		if healthCheckProperty != nil {
			providerSpecificProperties = append(providerSpecificProperties, *healthCheckProperty)
		}
		dnsEndpoint.Spec.Endpoints = append(dnsEndpoint.Spec.Endpoints, &externaldnsk8siov1alpha1.Endpoint{
			DNSName: rule.Host,
			Targets: externaldnsk8siov1alpha1.Targets{
				target,
			},
			RecordType:       "CNAME",
			SetIdentifier:    r.ClusterName,
			ProviderSpecific: providerSpecificProperties,
		},
		)
	}
}

func (r *IngressReconciler) reconcileDNSEntries(ctx context.Context, ingress netv1.Ingress, ownerRef metav1.OwnerReference) error {
	log := r.Log.WithValues("IngressName", ingress.ObjectMeta.Name).WithValues("IngressNamespace", ingress.ObjectMeta.Namespace)

	if !r.ingressAnnotationMatchFilter(ingress) {
		log.Info("Ingress object doesn't match annotation filter. Skipping")
		return nil
	}

	target, err := r.getTargetFromIngress(ingress)
	if err != nil {
		log.Info("Ingress object doesn't have target assigned. Skipping")
		return nil
	}

	// Reconcile uses this property that an ingress has a single matching dnsendpoint
	// with the same name. Shall this be changed, we should also change the Reconcile code
	var dnsEndpoint = &externaldnsk8siov1alpha1.DNSEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ingress.GetName(),
			Namespace: ingress.GetNamespace(),
		},
	}
	var f controllerutil.MutateFn = func() error {
		r.newDnsEndpoint(ctx, dnsEndpoint, target, ingress, ownerRef)
		return nil
	}
	_, err = ctrl.CreateOrUpdate(ctx, r.Client, dnsEndpoint, f)
	return err
}

func (r *IngressReconciler) endpointBeingDeleted(ctx context.Context, obj types.NamespacedName) bool {
	endpoint := externaldnsk8siov1alpha1.DNSEndpoint{}

	if err := r.Get(ctx, obj, &endpoint); err != nil {
		// If there was a problem, ignore it
		return false
	}

	return !endpoint.ObjectMeta.DeletionTimestamp.IsZero()
}

/*
in DNSEndpoint Object we set its weight based on hostname level. in case a host in an ingress has more than 1 service for different paths,
we will only set the weight to 0 if all services do not have backing pods.
*/
func (r *IngressReconciler) ingressRuleHasPods(ctx context.Context, namespace string, rule *netv1.IngressRule) bool {

	// there are some edge cases that the rules does not have HTTP property defined
	// keeping the same behavior
	if rule.HTTP == nil {
		return false
	}
	paths := rule.HTTP.Paths

	var servicesName = make(map[string]types.NamespacedName)

	for _, path := range paths {
		servicesName[path.Backend.Service.Name] = types.NamespacedName{Namespace: namespace, Name: path.Backend.Service.Name}
	}

	for _, svc := range servicesName {

		var endpoints v1.Endpoints

		if err := r.Get(ctx, svc, &endpoints); err != nil {
			log := r.Log.WithValues("EndpointName", svc)
			log.Error(err, "Unable to fetch endpoints")
			// ignore the error, going back to normal path
			if apierrors.IsNotFound(err) {
				return false
			}
			return true
		}

		if !endpoints.DeletionTimestamp.IsZero() {
			// If the endpoint is being deleted, it will eventually disappear, and ingresses will not have pods pretty soon.
			// Handling this case here helps reduce downtime in some cases
			return false
		}

		if !r.endpointsHasPods(&endpoints) {
			// When a single service does not have pods, this will instruct the weight to go for another place for the whole domain
			return false
		}
	}
	return true
}

func (r *IngressReconciler) endpointsHasPods(endpoints *v1.Endpoints) bool {
	if len(endpoints.Subsets) == 0 {
		return false
	}
	for _, subset := range endpoints.Subsets {
		if len(subset.Addresses) == 0 {
			return false
		}
	}
	return true
}

func (r *IngressReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("IngressName", req.NamespacedName)

	trafficStoreMetrics.DesiredWeight.Set(float64(trafficweight.Store.DesiredWeight))
	trafficStoreMetrics.CurrentWeight.Set(float64(trafficweight.Store.CurrentWeight))

	var ingress netv1.Ingress
	controller := true
	var ownerRef metav1.OwnerReference

	if err := r.Get(ctx, req.NamespacedName, &ingress); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("The ingress object does not exist. Ensuring the dns endpoint does not exist either")
			// the ingress was deleted, maybe the controller was offline meanwhile deleted?
			// anyhow, we should remove the associated resources if they exist
			// As defined in reconcileDNSEntries there is a single DNSEntry created per ingress.
			// Shall this change, we should change the logic
			err = r.Client.Delete(
				ctx,
				&externaldnsk8siov1alpha1.DNSEndpoint{
					ObjectMeta: metav1.ObjectMeta{
						Name:      req.Name,
						Namespace: req.Namespace,
					},
				},
			)
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}

		log.Info("Unable to fetch Ingress, skipping")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	ownerRef = metav1.OwnerReference{
		APIVersion: ingress.APIVersion,
		Kind:       ingress.Kind,
		Name:       ingress.GetName(),
		UID:        ingress.GetUID(),
		Controller: &controller,
	}

	if ingress.ObjectMeta.DeletionTimestamp.IsZero() { // Not being deleted
		// There maybe situations in which we receive an reconciliation event while
		// the given dnsendpoint is being deleted in which we may lose both the event and the change
		// to prevent it, reschedule the event if endpoint is being deleted
		if r.endpointBeingDeleted(ctx, types.NamespacedName{Name: ingress.Name, Namespace: ingress.Namespace}) {
			log.Info("DNS endpoint being removed, requeuing notification")
			return ctrl.Result{Requeue: true, RequeueAfter: 5 * time.Second}, nil
		}
		err := r.reconcileDNSEntries(ctx, ingress, ownerRef)
		if err != nil {
			log.Error(err, "Could not reconcile DNS endpoints")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func hasNetworkingV1(k8sClient client.Client) bool {
	ingList := netv1.IngressList{}
	return k8sClient.List(context.Background(), &ingList) == nil
}

func (r *IngressReconciler) SetupWithManager(mgr ctrl.Manager, events chan event.GenericEvent) error {
	var ing client.Object = &netv1.Ingress{}

	endpointMapper := &endpointsMapper{
		Client: r.Client,
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(ing).
		Watches(&v1.Endpoints{}, handler.EnqueueRequestsFromMapFunc(endpointMapper.mapToIngressRequests)).
		Owns(&externaldnsk8siov1alpha1.DNSEndpoint{}).
		WatchesRawSource(source.Channel[client.Object](events, &handler.EnqueueRequestForObject{})).
		Complete(r)
}

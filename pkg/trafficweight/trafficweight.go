package trafficweight

import (
	"context"
	"fmt"
	"time"

	log "github.com/go-logr/logr"
	netv1 "k8s.io/api/networking/v1"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

type TrafficWeightBackend interface {
	ReadWeight() (int, error)
	OnWeightUpdate(StoreConfig) error
}

func NewBackend(backendType, clusterName string, awsRegion string, tableName string, awsHealthCheckID string, logger log.Logger) (TrafficWeightBackend, error) {
	switch backendType {
	case "fake":
		return NewFakeBackend(logger), nil
	case "dynamoDB":
		return NewDynamodbBackend(logger, clusterName, awsRegion, tableName), nil
	default:
		return nil, fmt.Errorf("Not implemented")
	}
}

func enqueueReconcileEvents(events chan event.GenericEvent, c cache.Cache) error {
	var ingresses netv1.IngressList
	err := c.List(context.Background(), &ingresses, &client.ListOptions{})
	if err != nil {
		return err
	}
	for i := range ingresses.Items {
		// Provide a distinct object for each loop.
		// as generic event accepts a pointer, using the intuitive for _, ing := range ingresses.Items {
		// copies for every single ingress the object into the same ing value. Then, we would always
		// provide the same address to event.GenericEvent{Object: &ing} and hence would make future calls
		// to handle multiple times the same ingress, and skipping some of them, depending on the concurrence pattern
		// we could do "DeepCopys" instead but that would consume more memory
		// We take advantage of ingresses already have allocated all the required memory and
		// avoid duplicating it
		genEvent := event.GenericEvent{
			Object: &netv1.Ingress{
				ObjectMeta: *ingresses.Items[i].ObjectMeta.DeepCopy(),
			},
		}
		events <- genEvent
	}
	return nil
}

func doReconcile(backend TrafficWeightBackend, c cache.Cache, events chan event.GenericEvent) error {
	desiredWeight, err := backend.ReadWeight()
	if err != nil {
		return err
	}

	if Store.CurrentWeight != desiredWeight {
		Store.DesiredWeight = desiredWeight
		err = enqueueReconcileEvents(events, c)
		if err != nil {
			return err
		}
		Store.CurrentWeight = desiredWeight
		err = backend.OnWeightUpdate(Store)
		if err != nil {
			return err
		}
	}

	return nil
}

func ConfigReconcileLoop(backend TrafficWeightBackend, c cache.Cache, seconds time.Duration, log log.Logger, events chan event.GenericEvent) {
	ticker := time.NewTicker(seconds)
	quit := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				err := doReconcile(backend, c, events)
				if err != nil {
					log.Error(err, "Error updating ingress weight on store backend")
				}
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()
}

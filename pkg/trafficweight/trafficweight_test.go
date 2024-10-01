package trafficweight

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var testLogger logr.Logger = zap.New(zap.UseDevMode(true))

type fakeCache struct {
	ing *netv1.IngressList
}

func (c *fakeCache) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	return nil
}

func (c *fakeCache) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	o, ok := list.(*netv1.IngressList)
	if !ok {
		return fmt.Errorf("Fake cache only works with IngressList")
	}
	o.Items = c.ing.Items
	return nil
}

func (c *fakeCache) GetInformer(ctx context.Context, obj client.Object, opts ...cache.InformerGetOption) (cache.Informer, error) {
	return nil, nil
}

func (c *fakeCache) GetInformerForKind(ctx context.Context, gvk schema.GroupVersionKind, opts ...cache.InformerGetOption) (cache.Informer, error) {
	return nil, nil
}

func (c *fakeCache) RemoveInformer(ctx context.Context, obj client.Object) error {
	return nil
}

func (c *fakeCache) Start(context.Context) error {
	return nil
}

func (c *fakeCache) WaitForCacheSync(context.Context) bool {
	return true
}

func (c *fakeCache) IndexField(ctx context.Context, obj client.Object, field string, extractValue client.IndexerFunc) error {
	return nil
}

func TestNewBackend(t *testing.T) {
	backend, err := NewBackend("fake", "foo", "", "", "a-healthy-check-id", zap.New(zap.UseDevMode(true)))

	assert.Nil(t, err)
	assert.NotNil(t, backend)

	backend, err = NewBackend("foolanito", "foo", "", "", "a-healthy-check-id", zap.New(zap.UseDevMode(true)))

	assert.NotNil(t, err)
	assert.Nil(t, backend)

}

func Test_enqueueReconcileEvents(t *testing.T) {

	events := make(chan event.GenericEvent, 1)
	cache := &fakeCache{}

	inglist := netv1.IngressList{
		Items: []netv1.Ingress{
			netv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
				},
				Spec: netv1.IngressSpec{
					Rules: []netv1.IngressRule{
						netv1.IngressRule{
							Host: "foo.cheap.io",
						},
					},
				},
			},
		},
	}

	cache.ing = &inglist

	err := enqueueReconcileEvents(events, cache)

	assert.Nil(t, err)

	err = nil

	select {
	case myEvent := <-events:
		assert.Equal(t,
			inglist.Items[0].GetNamespace(),
			myEvent.Object.GetNamespace(),
		)
		assert.Equal(t,
			inglist.Items[0].GetName(),
			myEvent.Object.GetName(),
		)
	default:
		err = assert.AnError
	}

	assert.Nil(t, err)

	close(events)

}

type testBackend struct {
	weight  int
	updated int
	err     error
}

func (b *testBackend) ReadWeight() (int, error) {
	return b.weight, nil
}

func (b *testBackend) OnWeightUpdate(StoreConfig) error {
	b.updated++
	return b.err
}

func Test_doReconcile(t *testing.T) {
	events := make(chan event.GenericEvent, 1)
	cache := &fakeCache{}
	cache.ing = &netv1.IngressList{}

	fake := &testBackend{
		weight:  500,
		updated: 0,
		err:     nil,
	}

	// There was an weight change and backend was updated
	err := doReconcile(fake, cache, events)

	assert.Nil(t, err)
	assert.Equal(t, fake.updated, 1)

	// Nothing changes, there should not be further updates
	err = doReconcile(fake, cache, events)

	assert.Nil(t, err)
	assert.Equal(t, fake.updated, 1) // There were no event updates

	// If weight changes and the update event is not properly handled
	fake.err = assert.AnError
	fake.weight = 250
	err = doReconcile(fake, cache, events)

	assert.NotNil(t, err)
}

package trafficweight

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestNewCliBackend(t *testing.T) {

	fakeBackend := NewFakeBackend(zap.New(zap.UseDevMode(true)))

	Store.DesiredWeight = 200

	w, e := fakeBackend.ReadWeight()

	assert.Equal(t, w, 200)
	assert.Nil(t, e)
	assert.Equal(t, fakeBackend.OnWeightUpdate(StoreConfig{}), nil)
}

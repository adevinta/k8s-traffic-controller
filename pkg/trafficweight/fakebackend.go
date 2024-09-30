package trafficweight

import "github.com/go-logr/logr"

type FakeBackend struct {
	Log logr.Logger
}

func NewFakeBackend(logger logr.Logger) TrafficWeightBackend {
	backend := FakeBackend{Log: logger}
	return &backend
}

func (b *FakeBackend) ReadWeight() (int, error) {
	return Store.DesiredWeight, nil
}

func (b *FakeBackend) OnWeightUpdate(config StoreConfig) error {
	return nil
}

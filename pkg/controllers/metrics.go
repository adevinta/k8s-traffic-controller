package controllers

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

type TrafficStoreMetrics struct {
	DesiredWeight prometheus.Gauge
	CurrentWeight prometheus.Gauge
}

var (
	trafficStoreMetrics = TrafficStoreMetrics{
		DesiredWeight: prometheus.NewGauge(prometheus.GaugeOpts{
			// cluster_traffic_controller_ingress_weight_desired
			Namespace: "cluster",
			Subsystem: "traffic_controller",
			Name:      "ingress_weight_desired",
			Help:      "The desired weight of the ingress",
		}),
		CurrentWeight: prometheus.NewGauge(
			prometheus.GaugeOpts{
				// cluster_traffic_controller_ingress_weight_current
				Namespace: "cluster",
				Subsystem: "traffic_controller",
				Name:      "ingress_weight_current",
				Help:      "The current weight of the cluster",
			},
		),
	}
)

func init() {
	metrics.Registry.MustRegister(trafficStoreMetrics.DesiredWeight, trafficStoreMetrics.CurrentWeight)
}

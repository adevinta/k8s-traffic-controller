package trafficweight

type StoreConfig struct {
	DesiredWeight    int
	CurrentWeight    int
	AWSHealthCheckID string
}

var Store StoreConfig

package main

import (
	"flag"
	"log"
	"os"
	"time"

	"github.com/adevinta/k8s-traffic-controller/pkg/controllers"
	"github.com/adevinta/k8s-traffic-controller/pkg/trafficweight"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/event"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	// +kubebuilder:scaffold:imports
	logruslogr "github.com/adevinta/go-log-toolkit"
)

var (
	scheme   = controllers.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func main() {
	var metricsAddr string
	var clusterName string
	var awsRegion string
	var bindingDomain string
	var backendType string
	var annotationFilter string
	var enableLeaderElection bool
	var devMode bool
	var initialWeight int
	var tableName string
	var awsHealthCheckID string
	var annotationPrefix string

	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&clusterName, "cluster-name", "", "The name of the cluster")
	flag.StringVar(&awsRegion, "aws-region", "eu-west-1", "The AWS Region for route53 provider")
	flag.StringVar(&bindingDomain, "binding-domain", "", "The domain to bind for create DNS entries")
	flag.StringVar(&backendType, "backend-type", "fake", "The config backend to use. By default uses fake")
	flag.StringVar(&annotationFilter, "annotation-filter", "", "Given an annotation, filter which ingress objects react to")
	flag.StringVar(&tableName, "table-name", "traffic-controller", "table name to use when reading from dynamodb backend")
	flag.StringVar(&awsHealthCheckID, "aws-health-check-id", "", "AWS route53 healthcheck id used, it can be only one.  set to \"\" to disable healthchecks")
	flag.StringVar(&annotationPrefix, "annotation-prefix", "dns.adevinta.com", "The prefix for traffic-management annotations in ingress objects (e.g. dns.adevinta.io/traffic-weight)")

	flag.IntVar(&initialWeight, "initial-weight", 0, "DNS weight for this cluster")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&devMode, "dev-mode", false,
		"Enables development mode for local development")
	flag.Parse()

	ctrl.SetLogger(logruslogr.NewLogr(logruslogr.DefaultLogger))

	trafficweight.Store = trafficweight.StoreConfig{
		DesiredWeight:    initialWeight,
		CurrentWeight:    initialWeight,
		AWSHealthCheckID: awsHealthCheckID,
	}

	backend, err := trafficweight.NewBackend(backendType, clusterName, awsRegion, tableName, awsHealthCheckID, ctrl.Log.WithName("ConfigBackend"))
	if err != nil {
		// Move this to log.Fatal with a proper logger
		panic(err.Error())
	}

	desiredWeight, err := backend.ReadWeight()
	if err != nil {
		// Move this to log.Fatal with a proper logger
		log.Fatal(err, "Unable to read desired weight from backend")
	}

	// We do not do gradual changes for now. So current == desired
	trafficweight.Store.DesiredWeight = desiredWeight
	trafficweight.Store.CurrentWeight = desiredWeight

	backend.OnWeightUpdate(trafficweight.Store)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: metricsAddr,
		},
		LeaderElection:   enableLeaderElection,
		LeaderElectionID: "a5568bf5.dns.adevinta.com",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	events := make(chan event.GenericEvent)

	if err = (&controllers.IngressReconciler{
		Client:           mgr.GetClient(),
		Log:              ctrl.Log.WithName("controllers").WithName("Ingress"),
		Scheme:           mgr.GetScheme(),
		ClusterName:      clusterName,
		AWSRegion:        awsRegion,
		DevMode:          devMode,
		BindingDomain:    bindingDomain,
		AnnotationFilter: controllers.NewAnnotationFilter(annotationFilter),
		AnnotationPrefix: annotationPrefix,
	}).SetupWithManager(mgr, events); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Ingress")
		os.Exit(1)
	}

	trafficweight.ConfigReconcileLoop(backend, mgr.GetCache(), 20*time.Second, ctrl.Log.WithName("ReconcileLoop"), events)

	// +kubebuilder:scaffold:builder

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

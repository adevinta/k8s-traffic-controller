package controllers

import (
	"os"
	"testing"

	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
	"sigs.k8s.io/e2e-framework/support/kind"
)

var (
	testEnvConf *envconf.Config
	testEnv     env.Environment
)

func TestMain(m *testing.M) {
	kindClusterName := "traffic-controller-integration-tests"
	if os.Getenv("RUN_INTEGRATION_TESTS") == "true" {
		testEnvConf = envconf.New()
		testEnv = env.NewWithConfig(testEnvConf)

		kindCluster := kind.NewCluster(kindClusterName)

		testEnv.Setup(
			envfuncs.CreateCluster(kindCluster, kindClusterName),
		)
		exitVal := testEnv.Run(m)
		testEnv.Finish(
			envfuncs.DestroyCluster(kindClusterName),
		)
		os.Exit(exitVal)
	} else {
		os.Exit(m.Run())
	}
}

package main

import (
	"context"
	"fmt"
	"runtime/metrics"
	"time"

	circlerriov1alpha1 "github.com/octopipe/circlerr/internal/api/v1alpha1"
	"github.com/octopipe/circlerr/internal/cache"
	"github.com/octopipe/circlerr/internal/gitmanager"
	"github.com/octopipe/circlerr/internal/k8scontrollers"
	"github.com/octopipe/circlerr/internal/reconciler"
	"github.com/octopipe/circlerr/internal/template"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(circlerriov1alpha1.AddToScheme(scheme))
}

func main() {
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     ":8000",
		Port:                   9443,
		HealthProbeBindAddress: ":8001",
		LeaderElection:         false,
		LeaderElectionID:       "dec90f54.circlerr.io",
	})
	if err != nil {
		panic(err)
	}

	config := ctrl.GetConfigOrDie()
	discoveryClient := discovery.NewDiscoveryClientForConfigOrDie(config)
	dynamicClient := dynamic.NewForConfigOrDie(config)
	gitManager := gitmanager.NewManager(mgr.GetClient())
	templateManager := template.NewTemplate(mgr.GetClient())
	k8sReconciler := reconciler.NewReconciler(discoveryClient, dynamicClient, cache.NewInMemoryCache(), template.NewTemplate(mgr.GetClient()))

	err = k8sReconciler.Preload(context.Background())
	if err != nil {
		panic(err)
	}

	k8sCircleController := k8scontrollers.NewCircleController(
		mgr.GetClient(),
		mgr.GetScheme(),
		gitManager,
		templateManager,
		k8sReconciler,
	)
	if err := k8sCircleController.SetupWithManager(mgr); err != nil {
		panic(err)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		panic(err)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		panic(err)
	}

	go func() {
		ticker := time.NewTicker(time.Second * 5)
		for {
			<-ticker.C
			tmpMetrics()
		}
	}()

	fmt.Println("start butler controller")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		panic(err)
	}
}

func tmpMetrics() {
	descs := metrics.All()

	samples := make([]metrics.Sample, len(descs))
	for i := range samples {
		samples[i].Name = descs[i].Name
	}

	metrics.Read(samples)

	for _, sample := range samples {
		name, value := sample.Name, sample.Value

		switch value.Kind() {
		case metrics.KindUint64:
			fmt.Printf("%s: %d\n", name, value.Uint64())
		case metrics.KindFloat64:
			fmt.Printf("%s: %f\n", name, value.Float64())
		case metrics.KindFloat64Histogram:
			fmt.Printf("%s: %f\n", name, medianBucket(value.Float64Histogram()))
		case metrics.KindBad:
			panic("bug in runtime/metrics package!")
		default:
			fmt.Printf("%s: unexpected metric Kind: %v\n", name, value.Kind())
		}
	}

	fmt.Println("==============================")
}

func medianBucket(h *metrics.Float64Histogram) float64 {
	total := uint64(0)
	for _, count := range h.Counts {
		total += count
	}
	thresh := total / 2
	total = 0
	for i, count := range h.Counts {
		total += count
		if total >= thresh {
			return h.Buckets[i]
		}
	}
	panic("should not happen")
}

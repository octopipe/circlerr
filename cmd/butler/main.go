package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"runtime/metrics"

	"github.com/go-logr/zapr"
	"github.com/joho/godotenv"
	circlerriov1alpha1 "github.com/octopipe/circlerr/internal/api/v1alpha1"
	"github.com/octopipe/circlerr/internal/gitmanager"
	"github.com/octopipe/circlerr/internal/k8scontrollers"
	"github.com/octopipe/circlerr/internal/templatemanager"
	"github.com/octopipe/circlerr/internal/utils/annotation"
	"github.com/octopipe/circlerr/pkg/twice/cache"
	"github.com/octopipe/circlerr/pkg/twice/reconciler"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
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
	_ = godotenv.Load()
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     "0",
		Port:                   9443,
		HealthProbeBindAddress: ":8001",
		LeaderElection:         false,
		LeaderElectionID:       "dec90f54.circlerr.io",
	})
	if err != nil {
		panic(err)
	}

	logger, _ := zap.NewProduction()
	config := ctrl.GetConfigOrDie()
	exporter, err := prometheus.New()
	if err != nil {
		log.Fatal(err)
	}
	provider := metric.NewMeterProvider(metric.WithReader(exporter))
	_ = provider.Meter("github.com/octopipe/circlerr")
	gitManager := gitmanager.NewManager(mgr.GetClient())
	templateManager := templatemanager.NewTemplateManager(mgr.GetClient())
	clusterCache := cache.NewLocalCache()
	k8sReconciler := reconciler.NewReconciler(zapr.NewLogger(logger), config, clusterCache)

	err = k8sReconciler.Preload(context.Background(), func(un *unstructured.Unstructured) bool {
		a := un.GetAnnotations()
		isControlled := a[annotation.ControlledByAnnotation] == annotation.ControlledByAnnotationValue

		return isControlled
	}, true)
	if err != nil {
		panic(err)
	}

	k8sCircleController := k8scontrollers.NewCircleController(
		logger,
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
		http.Handle("/metrics", promhttp.Handler())
		fmt.Println("serving metrics....")
		err := http.ListenAndServe(":8000", nil)
		if err != nil {
			panic(err)
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

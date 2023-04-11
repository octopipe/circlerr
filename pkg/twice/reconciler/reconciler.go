package reconciler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/octopipe/circlerr/pkg/twice/cache"
	"github.com/octopipe/circlerr/pkg/twice/resource"
	"golang.org/x/sync/errgroup"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	clientCache "k8s.io/client-go/tools/cache"
	watchutil "k8s.io/client-go/tools/watch"
	"k8s.io/client-go/util/retry"
)

const (
	LastAppliedConfigurationAnnotation = "twice.io/last-applied-configuration"
)

const (
	PlanImmutableAction = "IMMUTABLE"
	PlanUpdateAction    = "UPDATE"
	PlanCreateAction    = "CREATE"
	PlanDeleteAction    = "DELETE"
)

var (
	ignoredResources = map[string]bool{
		"events": true,
	}
)

type Reconciler interface {
	Planner
	Preload(ctx context.Context, isManaged isManagedFunc, liveUpdate bool) error
	Apply(ctx context.Context, planResults []PlanResult, namespace string) ([]ApplyResult, error)
}

type PlanResult struct {
	resource.Resource
	Action         string
	SrcManifest    string
	TargetManifest string
	DiffString     []string
}

type ApplyResult struct {
	PlanResult
	Status string
	Err    error
}

type isManagedFunc func(un *unstructured.Unstructured) bool

type reconciler struct {
	Planner
	logger logr.Logger
	config *rest.Config
	cache  cache.Cache

	dynamicClient   *dynamic.DynamicClient
	discoveryClient *discovery.DiscoveryClient
}

func NewReconciler(logger logr.Logger, config *rest.Config, cache cache.Cache) Reconciler {
	dynamicClient := dynamic.NewForConfigOrDie(config)
	discoveryClient := discovery.NewDiscoveryClientForConfigOrDie(config)
	planner := NewPlanner(cache, discoveryClient)

	return reconciler{
		Planner:         planner,
		logger:          logger,
		config:          config,
		cache:           cache,
		dynamicClient:   dynamicClient,
		discoveryClient: discoveryClient,
	}
}

func isSupportedVerb(verbs []string) bool {
	foundList := false
	foundWatch := false
	for _, verb := range verbs {
		if verb == "list" {
			foundList = true
			continue
		}

		if verb == "watch" {
			foundWatch = true
			continue
		}
	}

	return foundList && foundWatch
}

func (r reconciler) syncCache(ctx context.Context, resourceList *v1.APIResourceList, apiResource v1.APIResource, liveUpdate bool, isManaged isManagedFunc) func() error {
	return func() error {
		gvk := schema.FromAPIVersionAndKind(resourceList.GroupVersion, apiResource.Kind)
		gvr := gvk.GroupVersion().WithResource(apiResource.Name)

		dynamicInterface := r.dynamicClient.Resource(gvr)
		uns, err := dynamicInterface.List(ctx, v1.ListOptions{})
		if err != nil {
			return err
		}

		for _, un := range uns.Items {
			isManaged := isManaged(&un)
			newRes := resource.NewResourceByUnstructured(un, un.GetNamespace(), apiResource.Name, isManaged)
			r.cache.Set(newRes.GetResourceIdentifier(), newRes)
		}

		if liveUpdate {
			go r.watch(ctx, uns.GetResourceVersion(), apiResource.Name, dynamicInterface, isManaged)
		}

		return nil
	}
}

func (r reconciler) watch(ctx context.Context, resourceVersion string, apiResourceName string, dynamicInterface dynamic.NamespaceableResourceInterface, isManaged isManagedFunc) {
	wait.PollImmediateUntil(time.Second*3, func() (bool, error) {
		w, err := watchutil.NewRetryWatcher(resourceVersion, &clientCache.ListWatch{
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				res, err := dynamicInterface.Watch(ctx, options)
				if k8sErrors.IsNotFound(err) {
					fmt.Println("RES NOT FOUND")
				}

				return res, err
			},
		})
		if err != nil {
			return false, err
		}

		defer w.Stop()

		for {
			select {
			case <-ctx.Done():
				return true, nil
			case <-w.Done():
				return false, errors.New("was done on init")
			case event, ok := <-w.ResultChan():
				if !ok {
					return false, errors.New("was closed on init")
				}

				obj, ok := event.Object.(*unstructured.Unstructured)
				if !ok {
					return false, errors.New("was closed")
				}

				res := resource.NewResourceByUnstructured(*obj, obj.GetNamespace(), apiResourceName, isManaged(obj))
				key := res.GetResourceIdentifier()
				if event.Type == watch.Deleted && r.cache.Has(key) {
					r.cache.Delete(key)
				} else {
					r.cache.Set(key, res)
				}
			}
		}
	}, ctx.Done())
}

func (r reconciler) Preload(ctx context.Context, isManaged isManagedFunc, liveUpdate bool) error {
	apiResouceList, err := r.discoveryClient.ServerPreferredResources()
	if err != nil {
		return err
	}

	g, _ := errgroup.WithContext(ctx)
	for _, resourceList := range apiResouceList {
		for _, apiResource := range resourceList.APIResources {
			if _, ok := ignoredResources[apiResource.Name]; ok || !isSupportedVerb(apiResource.Verbs) {
				continue
			}

			g.Go(r.syncCache(ctx, resourceList, apiResource, liveUpdate, isManaged))
		}
	}

	if err := g.Wait(); err != nil {
		return err
	}

	return nil
}

func (r reconciler) SetLastAppliedConfiguration(un *unstructured.Unstructured, newConfiguration string) *unstructured.Unstructured {
	currentAnnotations := un.GetAnnotations()

	if currentAnnotations == nil {
		currentAnnotations = make(map[string]string)
	}

	currentAnnotations[LastAppliedConfigurationAnnotation] = newConfiguration
	un.SetAnnotations(currentAnnotations)
	return un
}

// TODO: hook before apply is necessary?
func (r reconciler) Apply(ctx context.Context, planResults []PlanResult, namespace string) ([]ApplyResult, error) {
	result := []ApplyResult{}

	if len(planResults) <= 0 {
		return []ApplyResult{}, nil
	}

	for _, res := range planResults {
		newApplyResult := ApplyResult{PlanResult: res}

		dynamicInterface := r.dynamicClient.Resource(schema.GroupVersionResource{
			Group:    res.Group,
			Version:  res.Version,
			Resource: res.ResourceName,
		}).Namespace(namespace)

		if res.Action == PlanCreateAction {
			res.Object = r.SetLastAppliedConfiguration(res.Object, res.TargetManifest)
			_, err := dynamicInterface.Create(ctx, res.Object, v1.CreateOptions{})
			if err != nil {
				newApplyResult.Err = err
				result = append(result, newApplyResult)
				continue
			}

			r.cache.Set(res.GetResourceIdentifier(), res.Resource)
			result = append(result, newApplyResult)
		}

		if res.Action == PlanUpdateAction {
			res.Object = r.SetLastAppliedConfiguration(res.Object, res.TargetManifest)
			err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
				_, err := dynamicInterface.Update(ctx, res.Object, v1.UpdateOptions{})
				if err != nil {
					return err
				}

				return nil
			})

			if err != nil {
				newApplyResult.Err = err
				result = append(result, newApplyResult)
				continue
			}

			r.cache.Set(res.GetResourceIdentifier(), res.Resource)
			result = append(result, newApplyResult)
		}

		if res.Action == PlanDeleteAction {
			err := dynamicInterface.Delete(ctx, res.Name, v1.DeleteOptions{})
			if err != nil {
				newApplyResult.Err = err
				result = append(result, newApplyResult)
				continue
			}

			r.cache.Delete(res.GetResourceIdentifier())
			result = append(result, newApplyResult)
		}

	}

	return result, nil
}

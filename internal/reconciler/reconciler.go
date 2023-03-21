package reconciler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/octopipe/circlerr/internal/cache"
	"github.com/octopipe/circlerr/internal/resource"
	"github.com/octopipe/circlerr/internal/template"
	"github.com/sergi/go-diff/diffmatchpatch"
	"golang.org/x/sync/errgroup"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	clientCache "k8s.io/client-go/tools/cache"
	watchutil "k8s.io/client-go/tools/watch"
)

var (
	ignoredResources = map[string]bool{
		"events": true,
	}
)

const (
	ReconcileSuccess = "RECONCILE_SUCCESS"
	ReconcileFailed  = "RECONCILE_FAILED"
)

type ReconcileResourceResult struct {
	Resource resource.Resource
	Metadata map[string]string
	Status   string
	Error    string
}

type ReconcileResult struct {
	Resources []ReconcileResourceResult
	Status    string
	Error     string
}

type Reconciler interface {
	Preload(ctx context.Context, isManagedObject func(un *unstructured.Unstructured) bool) error
	Reconcile(
		ctx context.Context,
		objects []*unstructured.Unstructured,
		namespace string,
		isMatch func(un *unstructured.Unstructured) bool,
		addMetadata func(un *unstructured.Unstructured) map[string]string) (ReconcileResult, error)
}

type reconciler struct {
	dicoveryClient     *discovery.DiscoveryClient
	dynamicClient      *dynamic.DynamicClient
	cache              cache.Cache
	deserializer       runtime.Decoder
	dmp                *diffmatchpatch.DiffMatchPatch
	templateManager    template.Manager
	preferredResources *v1.APIResourceList
}

func NewReconciler(
	dicoveryClient *discovery.DiscoveryClient,
	dynamicClient *dynamic.DynamicClient,
	cache cache.Cache,
	templateManager template.Manager,
) Reconciler {
	return &reconciler{
		dicoveryClient:     dicoveryClient,
		dynamicClient:      dynamicClient,
		cache:              cache,
		deserializer:       clientgoscheme.Codecs.UniversalDeserializer(),
		dmp:                diffmatchpatch.New(),
		templateManager:    templateManager,
		preferredResources: &v1.APIResourceList{},
	}
}

func (r reconciler) Preload(ctx context.Context, isManagedObject func(un *unstructured.Unstructured) bool) error {
	apiResouceList, err := r.dicoveryClient.ServerPreferredResources()
	if err != nil {
		return err
	}

	g, _ := errgroup.WithContext(ctx)
	for _, resourceList := range apiResouceList {
		for _, resource := range resourceList.APIResources {
			if _, ok := ignoredResources[resource.Name]; ok || !isSupportedVerb(resource.Verbs) {
				continue
			}

			g.Go(r.syncCache(ctx, resourceList, resource, isManagedObject))
		}
	}

	if err := g.Wait(); err != nil {
		return err
	}

	return nil
}

func (r reconciler) getCurrentReconcileState(isMatch func(un *unstructured.Unstructured) bool) map[string]resource.Resource {
	filter := func(res resource.Resource) bool {
		return res.Object != nil && isMatch(res.Object)
	}
	return r.cache.Scan(filter)
}

func (r reconciler) modifyObjectInCluster(ctx context.Context, currentState map[string]resource.Resource, un *unstructured.Unstructured, resourceName string, namespace string) (resource.Resource, error) {
	gvr := un.GroupVersionKind().GroupVersion().WithResource(resourceName)
	dynamicResource := r.dynamicClient.Resource(gvr).Namespace(namespace)
	managedResource := resource.GetResourceByObject(*un, resourceName, namespace, true)

	fmt.Println(managedResource.GetKey(), currentState)

	if _, ok := currentState[managedResource.GetKey()]; !ok {
		fmt.Println("CREATE")
		_, err := dynamicResource.Create(ctx, un, v1.CreateOptions{})
		return resource.Resource{}, err
	}
	fmt.Println("UPDATE")
	_, err := dynamicResource.Update(ctx, un, v1.UpdateOptions{})
	return resource.Resource{}, err
}

func (r reconciler) deleteObjects(ctx context.Context, currentState map[string]resource.Resource, notDeleteResources map[string]bool, namespace string) error {
	for key, m := range currentState {
		if _, ok := notDeleteResources[key]; ok {
			continue
		}

		gvr := m.Object.GroupVersionKind().GroupVersion().WithResource(m.ResourceName)
		dynamicResource := r.dynamicClient.Resource(gvr).Namespace(namespace)
		err := dynamicResource.Delete(ctx, m.Name, v1.DeleteOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}

// TODO: make circle cluster state diff
func (r reconciler) Reconcile(
	ctx context.Context,
	objects []*unstructured.Unstructured,
	namespace string,
	isMatch func(un *unstructured.Unstructured) bool,
	addMetadata func(un *unstructured.Unstructured) map[string]string) (ReconcileResult, error) {

	resourcesResult := []ReconcileResourceResult{}
	modifiedResources := map[string]bool{}
	currentState := r.getCurrentReconcileState(isMatch)
	reconcileStatus := ReconcileSuccess
	reconcileError := ""

	for _, un := range objects {
		currentReconcileStatus := ReconcileSuccess
		currentReconcileError := ""

		resourceName, err := r.getServerResource(un)
		if err != nil {
			return ReconcileResult{}, err
		}

		managedResource, err := r.modifyObjectInCluster(ctx, currentState, un, resourceName, namespace)
		if err != nil {
			currentReconcileStatus = ReconcileFailed
			currentReconcileError = err.Error()
			reconcileStatus = ReconcileFailed
			reconcileError = err.Error()
		}

		modifiedResources[managedResource.GetKey()] = true
		resourcesResult = append(resourcesResult, ReconcileResourceResult{
			Resource: managedResource,
			Metadata: addMetadata(un),
			Status:   currentReconcileStatus,
			Error:    currentReconcileError,
		})

		if currentReconcileStatus == ReconcileSuccess {
			r.cache.Set(managedResource.GetKey(), managedResource)
		}
	}

	if reconcileStatus == ReconcileSuccess {
		err := r.deleteObjects(ctx, currentState, modifiedResources, namespace)
		if err != nil {
			reconcileStatus = ReconcileFailed
			reconcileError = err.Error()
		}
	}

	return ReconcileResult{
		Resources: resourcesResult,
		Status:    reconcileStatus,
		Error:     reconcileError,
	}, nil
}

func (r reconciler) syncCache(ctx context.Context, resourceList *v1.APIResourceList, apiResource v1.APIResource, isManagedObject func(un *unstructured.Unstructured) bool) func() error {
	return func() error {
		gvk := schema.FromAPIVersionAndKind(resourceList.GroupVersion, apiResource.Kind)
		gvr := gvk.GroupVersion().WithResource(apiResource.Name)

		dynamicInterface := r.dynamicClient.Resource(gvr)
		uns, err := dynamicInterface.List(ctx, v1.ListOptions{})
		if err != nil {
			return err
		}

		for _, un := range uns.Items {
			isManaged := isManagedObject(&un)
			newRes := resource.GetResourceByObject(un, apiResource.Name, "", isManaged)
			r.cache.Set(newRes.GetKey(), newRes)
		}

		// go r.watch(ctx, uns.GetResourceVersion(), apiResource.Name, dynamicInterface, isManagedObject)
		return nil
	}
}

func (r reconciler) watch(ctx context.Context, resourceVersion string, apiResourceName string, dynamicInterface dynamic.NamespaceableResourceInterface, isManagedObject func(un *unstructured.Unstructured) bool) {
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

				key := resource.GetResourceKeyByObject(obj)
				if event.Type == watch.Deleted && r.cache.Has(key) {
					r.cache.Delete(key)
				} else {
					newRes := resource.GetResourceByObject(*obj, apiResourceName, "", isManagedObject(obj))
					r.cache.Set(newRes.GetKey(), newRes)
				}
			}
		}
	}, ctx.Done())
}

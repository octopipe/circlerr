package reconciler

import (
	"context"
	"errors"
	"fmt"
	"time"

	circlerriov1alpha1 "github.com/octopipe/circlerr/internal/api/v1alpha1"
	"github.com/octopipe/circlerr/internal/cache"
	"github.com/octopipe/circlerr/internal/resource"
	"github.com/octopipe/circlerr/internal/template"
	"github.com/octopipe/circlerr/internal/utils/annotation"
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

type Reconciler interface {
	Preload(ctx context.Context) error
	Reconcile(ctx context.Context, circle circlerriov1alpha1.Circle, objects []*unstructured.Unstructured) ([]circlerriov1alpha1.CircleStatusResource, error)
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
	return reconciler{
		dicoveryClient:     dicoveryClient,
		dynamicClient:      dynamicClient,
		cache:              cache,
		deserializer:       clientgoscheme.Codecs.UniversalDeserializer(),
		dmp:                diffmatchpatch.New(),
		templateManager:    templateManager,
		preferredResources: &v1.APIResourceList{},
	}
}

func (r reconciler) Preload(ctx context.Context) error {
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

			g.Go(r.syncCache(ctx, resourceList, resource, IsManagedObject))
		}
	}

	if err := g.Wait(); err != nil {
		return err
	}

	fmt.Println(r.cache.List())

	return nil
}

func (r reconciler) Reconcile(ctx context.Context, circle circlerriov1alpha1.Circle, objects []*unstructured.Unstructured) ([]circlerriov1alpha1.CircleStatusResource, error) {
	mappedCircleResources := map[string]resource.ManagedResource{}
	appliedResources := map[string]string{}
	managedObjectsKeys := r.cache.ListManagedObjects()
	result := []circlerriov1alpha1.CircleStatusResource{}

	for _, key := range managedObjectsKeys {
		managedObject := r.cache.GetManagedObject(key)
		fmt.Println("MAPPED", managedObject.Unstructured.GetAnnotations())
		if r.isSameCircle(managedObject.Unstructured, circle.Name, circle.Namespace) {
			mappedCircleResources[key] = managedObject

		}
	}

	for _, un := range objects {
		resourceName, err := r.getServerResource(un)
		if err != nil {
			return nil, err
		}

		gvr := un.GroupVersionKind().GroupVersion().WithResource(resourceName)
		dynamicResource := r.dynamicClient.Resource(gvr).Namespace(circle.Spec.Namespace)
		managedResource := resource.ToManagedResource(un, resourceName)

		if _, ok := mappedCircleResources[managedResource.GetKey()]; !ok {

			_, err = dynamicResource.Create(ctx, un, v1.CreateOptions{})
			if err != nil {
				fmt.Println("CREATE APPLY", err)
				return nil, err
			}
		} else {
			_, err = dynamicResource.Update(ctx, un, v1.UpdateOptions{})
			if err != nil {
				fmt.Println("UPDATE APPLY", err)
				return nil, err
			}
		}

		appliedResources[managedResource.GetKey()] = ""
		r.cache.SetManagedObject(managedResource.GetKey(), managedResource)

		result = append(result, circlerriov1alpha1.CircleStatusResource{
			Group:     un.GroupVersionKind().Group,
			Kind:      un.GroupVersionKind().Kind,
			Namespace: un.GetNamespace(),
			Name:      un.GetName(),
			Status: circlerriov1alpha1.CircleResourceStatus{
				SyncedAt: time.Now().String(),
			},
			Module: circlerriov1alpha1.CircleResourceModule{
				Name:      un.GetAnnotations()[annotation.ModuleNameAnnotation],
				Namespace: un.GetAnnotations()[annotation.ModuleNamespaceAnnotation],
			},
		})
	}

	for key, m := range mappedCircleResources {
		if _, ok := appliedResources[key]; ok {
			continue
		}

		gvr := m.Unstructured.GroupVersionKind().GroupVersion().WithResource(m.ResourceName)
		dynamicResource := r.dynamicClient.Resource(gvr).Namespace(circle.Spec.Namespace)
		err := dynamicResource.Delete(ctx, m.Unstructured.GetName(), v1.DeleteOptions{})
		if err != nil {
			return nil, err
		}
	}

	return result, nil
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
			if IsManagedObject(&un) {
				newResource := resource.ToManagedResource(&un, apiResource.Name)
				r.cache.SetManagedObject(newResource.GetKey(), newResource)
			} else {
				newResource := resource.ToResource(&un, apiResource.Name)
				r.cache.Set(newResource.GetKey(), newResource)
			}

		}

		go r.watch(ctx, uns.GetResourceVersion(), apiResource.Name, dynamicInterface)
		return nil
	}
}

func (r reconciler) watch(ctx context.Context, resourceVersion string, apiResourceName string, dynamicInterface dynamic.NamespaceableResourceInterface) {
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
					if IsManagedObject(obj) {
						newResource := resource.ToManagedResource(obj, apiResourceName)
						r.cache.SetManagedObject(newResource.GetKey(), newResource)
					} else {
						newResource := resource.ToResource(obj, apiResourceName)
						r.cache.Set(newResource.GetKey(), newResource)
					}
				}
			}
		}
	}, ctx.Done())
}

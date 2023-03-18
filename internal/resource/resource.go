package resource

import (
	"fmt"

	"github.com/octopipe/circlerr/internal/utils/annotation"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type ResourceOwner struct {
	Name         string
	Kind         string
	Version      string
	IsController bool
}

type Resource struct {
	Name         string
	Group        string
	Kind         string
	Version      string
	ResourceName string
	Namespace    string
	Owners       []ResourceOwner
}

type ManagedResource struct {
	Resource
	Manifest     string
	Unstructured *unstructured.Unstructured
}

func (r Resource) GetKey() string {
	return fmt.Sprintf("name=%s;group=%s;kind=%s;namespace=%s", r.Name, r.Group, r.Kind, r.Namespace)
}

func ToResource(un *unstructured.Unstructured, resource string) Resource {
	owners := []ResourceOwner{}

	for _, o := range un.GetOwnerReferences() {
		isController := false

		if o.Controller != nil {
			isController = *o.Controller
		}

		owners = append(owners, ResourceOwner{
			Name:         o.Name,
			Kind:         o.Kind,
			IsController: isController,
			Version:      o.APIVersion,
		})
	}

	return Resource{
		Name:         un.GetName(),
		Group:        un.GroupVersionKind().Group,
		Kind:         un.GetKind(),
		Version:      un.GroupVersionKind().Version,
		ResourceName: resource,
		Namespace:    un.GetNamespace(),
		Owners:       owners,
	}
}
func ToManagedResource(un *unstructured.Unstructured, resource string) ManagedResource {
	owners := []ResourceOwner{}

	for _, o := range un.GetOwnerReferences() {
		isController := false

		if o.Controller != nil {
			isController = *o.Controller
		}

		owners = append(owners, ResourceOwner{
			Name:         o.Name,
			Kind:         o.Kind,
			IsController: isController,
			Version:      o.APIVersion,
		})
	}

	manifest := ""
	snapshot, ok := un.GetAnnotations()[annotation.SnapshotAnnotation]
	if ok {
		manifest = snapshot
	}

	return ManagedResource{
		Resource: Resource{
			Name:         un.GetName(),
			Group:        un.GroupVersionKind().Group,
			Kind:         un.GetKind(),
			Version:      un.GroupVersionKind().Version,
			ResourceName: resource,
			Namespace:    un.GetNamespace(),
			Owners:       owners,
		},
		Manifest:     manifest,
		Unstructured: un,
	}
}

func GetResourceKeyByObject(un *unstructured.Unstructured) string {
	return fmt.Sprintf("name=%s;group=%s;kind=%s;namespace=%s", un.GetName(), un.GroupVersionKind().Group, un.GetKind(), un.GetNamespace())
}

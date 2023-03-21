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
	IsManaged    bool
	Owners       []ResourceOwner
	Manifest     string
	Object       *unstructured.Unstructured
}

func (r Resource) GetKey() string {
	return fmt.Sprintf("name=%s;group=%s;kind=%s;version=%s;namespace=%s", r.Name, r.Group, r.Kind, r.Version, r.Namespace)
}

func GetResourceByObject(un unstructured.Unstructured, resource string, namespace string, isManaged bool) Resource {
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

	n := "default"
	if namespace != "" {
		n = namespace
	}

	newResource := Resource{
		Name:         un.GetName(),
		Group:        un.GroupVersionKind().Group,
		Kind:         un.GetKind(),
		Version:      un.GroupVersionKind().Version,
		ResourceName: resource,
		Namespace:    n,
		IsManaged:    isManaged,
		Owners:       owners,
	}

	if isManaged {
		snapshot, ok := un.GetAnnotations()[annotation.SnapshotAnnotation]
		if ok {
			newResource.Manifest = snapshot
		}

		newResource.Object = &un
	}

	return newResource
}

func GetResourceKeyByObject(un *unstructured.Unstructured) string {
	return fmt.Sprintf("name=%s;group=%s;kind=%s;version=%s;namespace=%s", un.GetName(), un.GroupVersionKind().Group, un.GetKind(), un.GroupVersionKind().Version, un.GetNamespace())
}

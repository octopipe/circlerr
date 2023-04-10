package resource

import (
	"fmt"

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
	Object       *unstructured.Unstructured
}

func NewResourceByUnstructured(un unstructured.Unstructured, namespace, resource string, isManaged bool) Resource {
	newResource := Resource{
		Name:         un.GetName(),
		Group:        un.GroupVersionKind().Group,
		Kind:         un.GetKind(),
		Version:      un.GroupVersionKind().Version,
		ResourceName: resource,
		Namespace:    namespace,
		Owners:       []ResourceOwner{},
	}

	if isManaged {
		newResource.Object = &un
	}

	return newResource
}

func (r Resource) GetResourceIdentifier() string {
	return fmt.Sprintf("name=%s;group=%s;version=%s;kind=%s;namespace=%s", r.Name, r.Group, r.Version, r.Kind, r.Namespace)
}

package reconciler

import (
	"errors"
	"strings"

	"github.com/octopipe/circlerr/internal/utils/annotation"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

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

func IsManagedObject(un *unstructured.Unstructured) bool {
	a := un.GetAnnotations()
	isControlled := a[annotation.ControlledByAnnotation] == annotation.ControlledByAnnotationValue

	return isControlled
}

func ParseRef(ref string) (string, string) {
	s := strings.Split(ref, "/")
	if len(s) == 1 {
		return "default", s[0]
	}

	return s[0], s[1]
}

func (r *reconciler) getServerResource(un *unstructured.Unstructured) (string, error) {
	apiResourceList, err := r.dicoveryClient.ServerResourcesForGroupVersion(un.GroupVersionKind().GroupVersion().String())
	if err != nil {
		return "", err
	}

	for _, apiResource := range apiResourceList.APIResources {
		if !isSupportedVerb(apiResource.Verbs) {
			continue
		}

		if apiResource.Kind == un.GetKind() {
			return apiResource.Name, nil
		}
	}

	return "", errors.New("server resource not supported")
}

func (r reconciler) isSameCircle(un *unstructured.Unstructured, circleName string, circleNamespace string) bool {
	objectAnnotations := un.GetAnnotations()

	circleNameAnnotation, ok := objectAnnotations[annotation.CircleNameAnnotation]
	if !ok {
		return false
	}

	circleNamespaceAnnotation, ok := objectAnnotations[annotation.CircleNamespaceAnnotation]
	if !ok {
		return false
	}

	return circleNameAnnotation == circleName && circleNamespaceAnnotation == circleNamespace
}

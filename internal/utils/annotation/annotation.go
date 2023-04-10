package annotation

import (
	circlerriov1alpha1 "github.com/octopipe/circlerr/internal/api/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	SnapshotAnnotation          = "circlerr.io/snapshot"
	ControlledByAnnotation      = "circlerr.io/controlled-by"
	ControlledByAnnotationValue = "circlerr.io"
	CircleNameAnnotation        = "circlerr.io/circle-name"
	CircleNamespaceAnnotation   = "circlerr.io/circle-namespace"
	ModuleNameAnnotation        = "circlerr.io/module-name"
	ModuleNamespaceAnnotation   = "circlerr.io/module-namespace"
	ModuleRevisionAnnotation    = "circlerr.io/module-revision"
)

func AddDefaultAnnotationsToObject(
	un *unstructured.Unstructured,
	circle circlerriov1alpha1.Circle,
) *unstructured.Unstructured {
	annotations := un.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}

	annotations[ControlledByAnnotation] = ControlledByAnnotationValue
	annotations[CircleNameAnnotation] = circle.GetName()
	annotations[CircleNamespaceAnnotation] = circle.GetNamespace()

	un.SetAnnotations(annotations)
	return un
}

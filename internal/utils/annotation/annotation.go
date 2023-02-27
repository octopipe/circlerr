package annotation

import (
	circlerriov1alpha1 "github.com/octopipe/circlerr/internal/api/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	SnapshotAnnotation        = "circlerr.io/snapshot"
	ControlledByAnnotation    = "circlerr.io/controlled-by"
	CircleNameAnnotation      = "circlerr.io/circle-name"
	CircleNamespaceAnnotation = "circlerr.io/circle-namespace"
	ModuleNameAnnotation      = "circlerr.io/module-name"
	ModuleNamespaceAnnotation = "circlerr.io/module-namespace"
)

func AddDefaultAnnotationsToObject(
	un *unstructured.Unstructured,
	module circlerriov1alpha1.Module,
	circle circlerriov1alpha1.Circle,
	snapshot string,
) *unstructured.Unstructured {
	annotations := un.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}

	annotations[SnapshotAnnotation] = snapshot
	annotations[ControlledByAnnotation] = "circlerr.io"
	annotations[CircleNameAnnotation] = circle.GetName()
	annotations[CircleNamespaceAnnotation] = circle.GetNamespace()
	annotations[ModuleNameAnnotation] = module.GetName()
	annotations[ModuleNamespaceAnnotation] = module.GetNamespace()

	un.SetAnnotations(annotations)
	return un
}

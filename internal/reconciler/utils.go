package reconciler

import (
	"strings"

	"github.com/octopipe/circlerr/internal/utils/annotation"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func IsSupportedVerb(verbs []string) bool {
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
	isControlled := a[annotation.ControlledByAnnotation] == "circlerr.io"

	return isControlled
}

func ParseRef(ref string) (string, string) {
	s := strings.Split(ref, "/")
	if len(s) == 1 {
		return "default", s[0]
	}

	return s[0], s[1]
}

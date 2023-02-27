package manifest

import (
	"bytes"
	"encoding/json"
	"io"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	kubeyaml "k8s.io/apimachinery/pkg/util/yaml"
)

func SplitManifests(manifest []byte) ([]string, error) {
	decoder := kubeyaml.NewYAMLOrJSONDecoder(bytes.NewReader(manifest), 4096)
	manifests := []string{}

	for {
		newManifest := ""
		err := decoder.Decode(&newManifest)
		if err == io.EOF {
			break
		}

		manifests = append(manifests, newManifest)
	}

	return manifests, nil
}

func ToUnstructured(manifest string) (*unstructured.Unstructured, error) {
	newManifest := &unstructured.Unstructured{}
	if err := json.Unmarshal([]byte(manifest), newManifest); err != nil {
		return nil, err
	}

	return newManifest, nil
}

package manifest

import (
	"bytes"
	"encoding/json"
	"io"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	kubeyaml "k8s.io/apimachinery/pkg/util/yaml"
)

func SplitManifests(manifest []byte) ([]string, error) {
	decoder := kubeyaml.NewYAMLOrJSONDecoder(bytes.NewReader(manifest), 4096)
	manifests := []string{}

	for {
		newManifest := runtime.RawExtension{}
		err := decoder.Decode(&newManifest)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		newManifest.Raw = bytes.TrimSpace(newManifest.Raw)
		if len(newManifest.Raw) == 0 || bytes.Equal(newManifest.Raw, []byte("null")) {
			continue
		}

		manifests = append(manifests, string(newManifest.Raw))
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

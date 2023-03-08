package template

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	circlerriov1alpha1 "github.com/octopipe/circlerr/internal/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type simpleTemplate struct {
	client.Client
}

func NewSimpleTemplate(client client.Client) Template {
	return simpleTemplate{Client: client}
}

func (t simpleTemplate) GetManifests(ctx context.Context, module circlerriov1alpha1.Module, circle circlerriov1alpha1.Circle) ([][]byte, error) {
	manifests := [][]byte{}

	deploymentPath := module.Spec.Path
	repositoryPath := fmt.Sprintf("%s/%s", os.Getenv("GIT_TMP_DIR"), module.Spec.Path)
	if err := filepath.Walk(filepath.Join(repositoryPath, deploymentPath), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if ext := strings.ToLower(filepath.Ext(info.Name())); ext != ".json" && ext != ".yml" && ext != ".yaml" {
			return nil
		}
		data, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}
		manifests = append(manifests, data)
		return nil
	}); err != nil {
		return nil, err
	}

	return manifests, nil
}

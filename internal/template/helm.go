package template

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	circlerriov1alpha1 "github.com/octopipe/circlerr/internal/api/v1alpha1"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type helmTemplate struct {
	client.Client
}

func NewHelmTemplate(client client.Client) Template {
	return helmTemplate{Client: client}
}

func (t helmTemplate) GetManifests(ctx context.Context, module circlerriov1alpha1.Module, circle circlerriov1alpha1.Circle) ([][]byte, error) {
	settings := cli.New()

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), circle.Namespace,
		os.Getenv("HELM_DRIVER"), log.Printf); err != nil {
		log.Printf("%+v", err)
		os.Exit(1)
	}

	vals := map[string]interface{}{}
	repositoryPath := fmt.Sprintf("%s/%s", os.Getenv("GIT_TMP_DIR"), module.Spec.Path)
	chart, err := loader.Load(filepath.Join(repositoryPath, module.Spec.Path))
	if err != nil {
		panic(err)
	}

	client := action.NewInstall(actionConfig)
	client.Namespace = circle.Spec.Namespace
	client.ReleaseName = module.Name
	client.DryRun = true
	client.Devel = true
	client.Replace = true
	client.ClientOnly = true

	values, err := client.Run(chart, vals)
	if err != nil {
		panic(err)
	}

	manifest := []byte(values.Manifest)
	manifests := [][]byte{manifest}

	return manifests, nil
}

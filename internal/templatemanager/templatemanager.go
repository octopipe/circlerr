package templatemanager

import (
	"context"
	"errors"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/parser"
	circlerriov1alpha1 "github.com/octopipe/circlerr/internal/api/v1alpha1"
	"github.com/octopipe/circlerr/internal/domain"
	"github.com/octopipe/circlerr/internal/utils/manifest"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Template interface {
	GetManifests(ctx context.Context, module circlerriov1alpha1.Module, circle circlerriov1alpha1.Circle) ([][]byte, error)
}

type TemplateManager struct {
	client.Client
	simpleTemplate Template
	helmTemplate   Template
}

func NewTemplateManager(client client.Client) TemplateManager {
	return TemplateManager{
		Client:         client,
		simpleTemplate: NewSimpleTemplate(client),
		helmTemplate:   NewHelmTemplate(client),
	}
}

func (t TemplateManager) RenderManifests(ctx context.Context, circle circlerriov1alpha1.Circle) ([]string, error) {
	manifests := []string{}

	for _, circleModule := range circle.Spec.Modules {
		module := &circlerriov1alpha1.Module{}
		moduleKey := types.NamespacedName{Namespace: circleModule.Namespace, Name: circleModule.Name}
		err := t.Get(ctx, moduleKey, module)
		if err != nil {
			return nil, err
		}

		rawManifests, err := t.getManifests(ctx, *module, circle)
		if err != nil {
			return nil, err
		}

		for _, r := range rawManifests {

			splitedManifests, err := manifest.SplitManifests(r)
			if err != nil {
				return nil, err
			}

			for _, m := range splitedManifests {
				m, err := t.overrideValues(m, circleModule.Overrides)
				if err != nil {
					return nil, err
				}
				manifests = append(manifests, m)

			}
		}
	}

	return manifests, nil
}

func (t TemplateManager) overrideValues(manifest string, overrides []circlerriov1alpha1.Override) (string, error) {
	file, err := parser.ParseBytes([]byte(manifest), 1)
	if err != nil {
		return "", err
	}

	for _, override := range overrides {
		p, err := yaml.PathString(override.Key)
		if err != nil {
			return "", err
		}

		node, err := yaml.NewEncoder(nil, yaml.JSON()).EncodeToNode(override.Value)
		if err != nil {
			return "", err
		}

		err = p.ReplaceWithNode(file, node)
		if err != nil {
			return "", err
		}
	}

	return file.String(), nil
}

func (t TemplateManager) getManifests(ctx context.Context, module circlerriov1alpha1.Module, circle circlerriov1alpha1.Circle) ([][]byte, error) {
	switch module.Spec.TemplateType {
	case domain.SimpleModuleTemplateType:
		return t.simpleTemplate.GetManifests(ctx, module, circle)
	case domain.HelmModuleTemplateType:
		return t.helmTemplate.GetManifests(ctx, module, circle)
	default:
		return nil, errors.New("invalid module type")
	}
}

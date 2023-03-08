package template

import (
	"context"
	"errors"
	"fmt"

	circlerriov1alpha1 "github.com/octopipe/circlerr/internal/api/v1alpha1"
	"github.com/octopipe/circlerr/internal/domain"
	"github.com/octopipe/circlerr/internal/utils/annotation"
	"github.com/octopipe/circlerr/internal/utils/manifest"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Template interface {
	GetManifests(ctx context.Context, module circlerriov1alpha1.Module, circle circlerriov1alpha1.Circle) ([][]byte, error)
}

type Manager struct {
	client.Client
	simpleTemplate Template
	helmTemplate   Template
}

func NewTemplate(client client.Client) Manager {
	return Manager{
		Client:         client,
		simpleTemplate: NewSimpleTemplate(client),
		helmTemplate:   NewHelmTemplate(client),
	}
}

func (t Manager) GetObjects(ctx context.Context, circle circlerriov1alpha1.Circle) (map[circlerriov1alpha1.CircleModuleKey][]*unstructured.Unstructured, error) {
	objects := map[circlerriov1alpha1.CircleModuleKey][]*unstructured.Unstructured{}

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
				object, err := manifest.ToUnstructured(m)
				if err != nil {
					return nil, err
				}

				object.SetName(fmt.Sprintf("%s-%s", circle.GetName(), object.GetName()))
				object = annotation.AddDefaultAnnotationsToObject(object, *module, circle, m)
				circleModuleKey := circlerriov1alpha1.CircleModuleKey{
					Name:      circleModule.Name,
					Namespace: circleModule.Namespace,
					Revision:  circleModule.Revision,
				}
				objects[circleModuleKey] = append(objects[circleModuleKey], object)
			}
		}
	}

	return objects, nil
}

func (t Manager) getManifests(ctx context.Context, module circlerriov1alpha1.Module, circle circlerriov1alpha1.Circle) ([][]byte, error) {
	switch module.Spec.TemplateType {
	case domain.SimpleModuleTemplateType:
		return t.simpleTemplate.GetManifests(ctx, module, circle)
	case domain.HelmModuleTemplateType:
		return t.helmTemplate.GetManifests(ctx, module, circle)
	default:
		return nil, errors.New("invalid module type")
	}
}

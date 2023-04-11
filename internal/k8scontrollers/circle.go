package k8scontrollers

import (
	"context"
	"fmt"
	"time"

	circlerriov1alpha1 "github.com/octopipe/circlerr/internal/api/v1alpha1"
	"github.com/octopipe/circlerr/internal/gitmanager"
	"github.com/octopipe/circlerr/internal/templatemanager"
	"github.com/octopipe/circlerr/internal/utils/annotation"
	"github.com/octopipe/circlerr/pkg/twice/reconciler"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type CircleController interface {
	Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error)
}

type circleController struct {
	client.Client
	logger          *zap.Logger
	scheme          *runtime.Scheme
	reconciler      reconciler.Reconciler
	gitManager      gitmanager.Manager
	templateManager templatemanager.TemplateManager
}

func NewCircleController(
	logger *zap.Logger,
	client client.Client,
	scheme *runtime.Scheme,
	gitManager gitmanager.Manager,
	templateManager templatemanager.TemplateManager,
	reconciler reconciler.Reconciler,
) circleController {
	return circleController{
		logger:          logger,
		Client:          client,
		scheme:          scheme,
		reconciler:      reconciler,
		templateManager: templateManager,
		gitManager:      gitManager,
	}
}

func (r *circleController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	circle := circlerriov1alpha1.Circle{}
	err := r.Get(ctx, req.NamespacedName, &circle)
	if err != nil {
		return ctrl.Result{}, err
	}

	applyResults := []reconciler.ApplyResult{}
	if len(circle.Finalizers) > 0 {
		r.logger.Info(fmt.Sprintf("circle %s with finalizers", req), zap.Any("finalizers", circle.Finalizers))
		applyResults, err = r.forDeletion(ctx, circle)
		if err != nil {
			return ctrl.Result{}, err
		}
	} else {
		r.logger.Info(fmt.Sprintf("apply circle %s", req))
		applyResults, err = r.forApply(ctx, circle)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	resourceStatus := []circlerriov1alpha1.CircleStatusResource{}
	for _, res := range applyResults {
		resourceStatus = append(resourceStatus, circlerriov1alpha1.CircleStatusResource{
			Group:     res.Resource.Group,
			Kind:      res.Resource.Kind,
			Name:      res.Resource.Name,
			Namespace: res.Resource.Namespace,
			Status: circlerriov1alpha1.CircleResourceStatus{
				SyncStatus: res.Status,
				SyncedAt:   time.Now().String(),
			},
		})
	}

	circle.Status = circlerriov1alpha1.CircleStatus{
		Resources: resourceStatus,
	}
	// err = r.Status().Update(ctx, &circle)
	// if err != nil {
	// 	return ctrl.Result{}, err
	// }

	return ctrl.Result{}, nil
}

func (r circleController) forApply(ctx context.Context, circle circlerriov1alpha1.Circle) ([]reconciler.ApplyResult, error) {
	for _, m := range circle.Spec.Modules {
		module := circlerriov1alpha1.Module{}
		key := types.NamespacedName{Namespace: m.Namespace, Name: m.Name}
		err := r.Get(ctx, key, &module)
		if err != nil {
			return nil, err
		}

		err = r.gitManager.Sync(module)
		if err != nil {
			return nil, err
		}

	}

	manifests, err := r.templateManager.RenderManifests(ctx, circle)
	if err != nil {
		return nil, err
	}

	preHook := func(un *unstructured.Unstructured) *unstructured.Unstructured {
		un.SetName(fmt.Sprintf("%s-%s", circle.GetName(), un.GetName()))
		un = annotation.AddDefaultAnnotationsToObject(un, circle)
		return un
	}

	planResults, err := r.reconciler.Plan(ctx, manifests, circle.Spec.Namespace, func(un *unstructured.Unstructured) bool {
		circleName := un.GetAnnotations()[annotation.CircleNameAnnotation]
		circleNamespace := un.GetAnnotations()[annotation.CircleNamespaceAnnotation]

		return circleName == circle.Name && circleNamespace == circle.Namespace
	}, reconciler.WithPreHook(preHook))
	if err != nil {
		return nil, err
	}

	applyResults, err := r.reconciler.Apply(ctx, planResults, circle.Spec.Namespace)
	return applyResults, err
}

func (r circleController) forDeletion(ctx context.Context, circle circlerriov1alpha1.Circle) ([]reconciler.ApplyResult, error) {
	planResults, err := r.reconciler.Plan(ctx, []string{}, circle.Spec.Namespace, func(un *unstructured.Unstructured) bool {
		circleName := un.GetAnnotations()[annotation.CircleNameAnnotation]
		circleNamespace := un.GetAnnotations()[annotation.CircleNamespaceAnnotation]

		return circleName == circle.Name && circleNamespace == circle.Namespace
	})
	if err != nil {
		return nil, err
	}

	applyResults, err := r.reconciler.Apply(ctx, planResults, circle.Spec.Namespace)
	return applyResults, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *circleController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&circlerriov1alpha1.Circle{}).
		Complete(r)
}

package k8scontrollers

import (
	"context"
	"fmt"
	"time"

	circlerriov1alpha1 "github.com/octopipe/circlerr/internal/api/v1alpha1"
	"github.com/octopipe/circlerr/internal/gitmanager"
	"github.com/octopipe/circlerr/internal/reconciler"
	"github.com/octopipe/circlerr/internal/template"
	"github.com/octopipe/circlerr/internal/utils/annotation"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type CircleController interface {
	Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error)
}

type circleController struct {
	client.Client
	scheme          *runtime.Scheme
	reconciler      reconciler.Reconciler
	gitManager      gitmanager.Manager
	templateManager template.Manager
}

func NewCircleController(
	client client.Client,
	scheme *runtime.Scheme,
	gitManager gitmanager.Manager,
	templateManager template.Manager,
	reconciler reconciler.Reconciler,
) circleController {
	return circleController{
		Client:          client,
		scheme:          scheme,
		reconciler:      reconciler,
		templateManager: templateManager,
		gitManager:      gitManager,
	}
}

func (r *circleController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	circle := circlerriov1alpha1.Circle{}
	err := r.Get(ctx, req.NamespacedName, &circle)
	if err != nil {
		return ctrl.Result{}, err
	}

	for _, m := range circle.Spec.Modules {
		module := circlerriov1alpha1.Module{}
		key := types.NamespacedName{Namespace: m.Namespace, Name: m.Name}
		err = r.Get(ctx, key, &module)
		if err != nil {
			return ctrl.Result{}, err
		}

		err = r.gitManager.Sync(module)
		if err != nil {
			return ctrl.Result{}, err
		}

	}

	objects, err := r.templateManager.GetObjects(ctx, circle)
	if err != nil {
		fmt.Println(err)
		return ctrl.Result{}, err
	}

	reconcileResult, err := r.reconciler.Reconcile(
		ctx,
		objects,
		circle.Spec.Namespace,
		func(un *unstructured.Unstructured) bool {
			circleName := un.GetAnnotations()[annotation.CircleNameAnnotation]
			circleNamespace := un.GetAnnotations()[annotation.CircleNamespaceAnnotation]

			return circleName == circle.Name && circleNamespace == circle.Namespace
		},
		func(un *unstructured.Unstructured) map[string]string {
			return map[string]string{
				"moduleName":      un.GetAnnotations()[annotation.ModuleNameAnnotation],
				"moduleNamespace": un.GetAnnotations()[annotation.ModuleNamespaceAnnotation],
			}
		},
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	resourceStatus := []circlerriov1alpha1.CircleStatusResource{}
	for _, res := range reconcileResult.Resources {
		resourceStatus = append(resourceStatus, circlerriov1alpha1.CircleStatusResource{
			Group:     res.Resource.Group,
			Kind:      res.Resource.Kind,
			Name:      res.Resource.Name,
			Namespace: res.Resource.Namespace,
			Status: circlerriov1alpha1.CircleResourceStatus{
				SyncStatus: res.Status,
				SyncedAt:   time.Now().String(),
			},
			Module: circlerriov1alpha1.CircleResourceModule{
				Name:      res.Metadata["moduleName"],
				Namespace: res.Metadata["moduleNamespace"],
			},
		})
	}

	circle.Status = circlerriov1alpha1.CircleStatus{
		Resources:  resourceStatus,
		SyncStatus: reconcileResult.Status,
		Error:      reconcileResult.Error,
	}
	// err = r.Status().Update(ctx, &circle)
	// if err != nil {
	// 	return ctrl.Result{}, err
	// }

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *circleController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&circlerriov1alpha1.Circle{}).
		Complete(r)
}

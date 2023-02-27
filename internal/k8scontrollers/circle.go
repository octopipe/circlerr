package k8scontrollers

import (
	"context"

	circlerriov1alpha1 "github.com/octopipe/circlerr/internal/api/v1alpha1"
	"github.com/octopipe/circlerr/internal/reconciler"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type CircleController interface {
	Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error)
}

type circleController struct {
	client.Client
	scheme     *runtime.Scheme
	reconciler reconciler.Reconciler
}

func NewCircleController(
	client client.Client,
	scheme *runtime.Scheme,
	reconciler reconciler.Reconciler,
) circleController {
	return circleController{
		Client:     client,
		scheme:     scheme,
		reconciler: reconciler,
	}
}

func (r *circleController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	r.reconciler.Reconcile(ctx, circlerriov1alpha1.Circle{})

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *circleController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&circlerriov1alpha1.Circle{}).
		Complete(r)
}

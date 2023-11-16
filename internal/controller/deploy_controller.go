/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	demov1 "demo-operator/api/v1"
	"demo-operator/helper"
	"log/slog"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// DeployReconciler reconciles a Deploy object
type DeployReconciler struct {
	client.Client
	Scheme      *runtime.Scheme
	EventRecord record.EventRecorder
}

//+kubebuilder:rbac:groups=demo.wsp.com,resources=deploys,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=demo.wsp.com,resources=deploys/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=demo.wsp.com,resources=deploys/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Deploy object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.15.0/pkg/reconcile
func (r *DeployReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	deploy := &demov1.Deploy{}
	if err := r.Get(ctx, req.NamespacedName, deploy); err != nil {
		slog.Info("no deploy")
		return ctrl.Result{}, nil
	} else {
		//  如果不为空 则正在删除
		if !deploy.DeletionTimestamp.IsZero() {
			return ctrl.Result{}, r.clearDeploy(ctx, deploy)
		}
		slog.Info("得到对象", "spec", deploy.Spec)
		podName := deploy.Name
		err := helper.CreateDeploy(r.Client, deploy, podName, r.Scheme)
		if err != nil {
			return ctrl.Result{}, err
		}
		if controllerutil.ContainsFinalizer(deploy, podName) {
			return ctrl.Result{}, nil
		}
		deploy.Finalizers = append(deploy.Finalizers, podName)
		err = r.Client.Update(ctx, deploy)
		if err != nil {
			return ctrl.Result{}, err
		}
		err = r.Status().Update(ctx, deploy)
		return ctrl.Result{}, err
	}
}

func (r *DeployReconciler) clearDeploy(ctx context.Context, deploy *demov1.Deploy) error {
	podList := deploy.Finalizers
	for _, podName := range podList {
		err := r.Client.Delete(ctx, &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      podName,
				Namespace: deploy.Namespace,
			},
		})

		if err != nil {
			slog.Error("清除Pod异常", "err", err)
		}
	}

	deploy.Finalizers = []string{}
	return r.Client.Update(ctx, deploy)
}

func (r *DeployReconciler) podDeleteHandler(ctx context.Context, event event.DeleteEvent, limitInterface workqueue.RateLimitingInterface) {
	slog.Info("被删除的对象", "name", event.Object.GetName())

	for _, ref := range event.Object.GetOwnerReferences() {
		// 因为会获取到所有被删除的pod，所以进行一次判断
		if ref.Kind == "Deploy" && ref.APIVersion == "demo.wsp.com/v1" {
			// 重新推送队列，进行 reconcile
			limitInterface.Add(reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      ref.Name,
					Namespace: event.Object.GetNamespace(),
				},
			})
		}
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *DeployReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&demov1.Deploy{}).
		Watches(&corev1.Pod{}, handler.Funcs{DeleteFunc: r.podDeleteHandler}).
		Complete(r)
}

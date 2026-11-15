/*
Copyright 2026.

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

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	webv1 "d7kj.com/nginx-operator/api/v1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// NginxReconciler reconciles a Nginx object
type NginxReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=web.d7kj.com,resources=nginxes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=web.d7kj.com,resources=nginxes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=web.d7kj.com,resources=nginxes/finalizers,verbs=update
// +kubebuilder:rbac:groups=web.d7kj.com,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=web.d7kj.com,resources=deployments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=web.d7kj.com,resources=deployments/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Nginx object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.4/pkg/reconcile
func (r *NginxReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	// TODO(user): your logic here
	// 1. 从 Kubernetes 获取Nginx CR
	nginx := &webv1.Nginx{}
	if err := r.Get(ctx, req.NamespacedName, nginx); err != nil {
		// 如果资源不存在，忽略 NotFound 错误
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// 2. 读取用户配置
	spec := &nginx.Spec
	replicas := int32(2)
	if spec.Replicas != nil {
		replicas = *spec.Replicas
	}
	image := spec.Image
	if image == "" {
		image = "nginx:1.27"
	}

	port := spec.Port
	if port == 0 {
		port = 80
	}

	labels := map[string]string{
		"app": nginx.Name,
	}

	// 3. 创建或更新 Deployment
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nginx.Name,
			Namespace: nginx.Namespace,
		},
	}
	_, err := controllerutil.CreateOrUpdate(
		ctx,
		r.Client,
		deployment,
		func() error {
			deployment.Labels = labels
			deployment.Spec.Replicas = &replicas
			deployment.Spec.Selector = &metav1.LabelSelector{
				MatchLabels: labels,
			}
			deployment.Spec.Template.Labels = labels
			deployment.Spec.Template.Spec.Containers = []corev1.Container{
				{
					Name:  "nginx",
					Image: image,
					Ports: []corev1.ContainerPort{
						{
							ContainerPort: port,
						},
					},
				},
			}
			return controllerutil.SetControllerReference(
				nginx,
				deployment,
				r.Scheme,
			)
		},
	)
	if err != nil {
		return ctrl.Result{}, err
	}
	// 4. 创建或更新Service
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nginx.Name,
			Namespace: nginx.Namespace,
		},
	}
	_, err = controllerutil.CreateOrUpdate(
		ctx,
		r.Client,
		service,
		func() error {
			service.Labels = labels
			service.Spec.Selector = labels
			service.Spec.Type = corev1.ServiceTypeClusterIP
			service.Spec.Ports = []corev1.ServicePort{
				{
					Name:       "http",
					Port:       port,
					TargetPort: intstr.FromInt32(port),
				},
			}
			return controllerutil.SetControllerReference(
				nginx,
				service,
				r.Scheme,
			)
		},
	)
	if err != nil {
		return ctrl.Result{}, err
	}
	// 5.更新 Nginx Status
	ready := deployment.Status.ReadyReplicas
	if nginx.Status.ReadyReplicas != ready {
		nginx.Status.ReadyReplicas = ready
		if err := r.Status().Update(ctx, nginx); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NginxReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&webv1.Nginx{}).
		Named("nginx").
		Complete(r)
}

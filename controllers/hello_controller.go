/*


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

package controllers

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	configv1alpha1 "github.com/ltmleo/hello-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// HelloReconciler reconciles a Hello object
type HelloReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=config.ltmleo.com,resources=hellos,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=config.ltmleo.com,resources=hellos/status,verbs=get;update;patch

func (r *HelloReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	_ = r.Log.WithValues("hello", req.NamespacedName)

	// Fetch the Presentation instance
	instance := &configv1alpha1.Hello{}
	err := r.Client.Get(context.TODO(), req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	configMapChanged, err := r.ensureLatestConfigMap(instance)
	if err != nil {
		return ctrl.Result{}, err
	}
	err = r.ensureLatestPod(instance, configMapChanged)
	if err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *HelloReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&configv1alpha1.Hello{}).
		Complete(r)
}

func (r *HelloReconciler) ensureLatestConfigMap(instance *configv1alpha1.Hello) (bool, error) {
	configMap := newConfigMap(instance)

	// Set Presentation instance as the owner and controller
	if err := controllerutil.SetControllerReference(instance, configMap, r.Scheme); err != nil {
		return false, err
	}

	// Check if this ConfigMap already exists
	foundMap := &corev1.ConfigMap{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: configMap.Name, Namespace: configMap.Namespace}, foundMap)
	if err != nil && errors.IsNotFound(err) {
		err = r.Client.Create(context.TODO(), configMap)
		if err != nil {
			return false, err
		}
	} else if err != nil {
		return false, err
	}

	if foundMap.Data["slides.md"] != configMap.Data["slides.md"] {
		err = r.Client.Update(context.TODO(), configMap)
		if err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

func newConfigMap(cr *configv1alpha1.Hello) *corev1.ConfigMap {
	labels := map[string]string{
		"app": cr.Name,
	}
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name + "-config",
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Data: map[string]string{
			"slides.md": cr.Spec.Markdown,
		},
	}
}

func (r *HelloReconciler) ensureLatestPod(instance *configv1alpha1.Hello, configMapChanged bool) error {
	// Define a new Pod object
	pod := newPodForCR(instance)

	// Set Presentation instance as the owner and controller
	if err := controllerutil.SetControllerReference(instance, pod, r.Scheme); err != nil {
		return err
	}
	// Check if this Pod already exists
	found := &corev1.Pod{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		err = r.Client.Create(context.TODO(), pod)
		time.Sleep(10 * time.Second)
		if err != nil {
			return err
		}

		// Pod created successfully - don't requeue
		return nil
	} else if err != nil {
		return err
	}

	if configMapChanged {
		err = r.Client.Delete(context.TODO(), found)
		if err != nil {
			return err
		}
		time.Sleep(10 * time.Second)
		err = r.Client.Create(context.TODO(), pod)
		if err != nil {
			return err
		}
		time.Sleep(10 * time.Second)
	}
	return nil
}

// newPodForCR returns a busybox pod with the same name/namespace as the cr
func newPodForCR(cr *configv1alpha1.Hello) *corev1.Pod {
	labels := map[string]string{
		"app": cr.Name,
	}
	volumeName := cr.Name + "-config"
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name + "-pod",
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "slides",
					Image: "manueldewald/presentation",
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      volumeName,
							MountPath: "/config",
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: volumeName,
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: cr.Name + "-config",
							},
						},
					},
				},
			},
		},
	}
}

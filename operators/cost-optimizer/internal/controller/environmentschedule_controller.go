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
	"fmt"
	"strconv"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	opsv1alpha1 "github.com/jgou/kube-components/operators/cost-optimizer/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
)

const (
	// CostOptimizerSkipLabel is the label key used to indicate that a deployment should be skipped by the cost optimizer.
	CostOptimizerSkipLabel = "cost-optimizer/skip"
	// CostOptimizerPreviousReplicasAnnotation is the annotation key used to store the previous number of replicas of a deployment before it was scaled down.
	CostOptimizerPreviousReplicasAnnotation = "cost-optimizer/previous-replicas"
)

// EnvironmentScheduleReconciler reconciles a EnvironmentSchedule object
type EnvironmentScheduleReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=ops.gouster.io,resources=environmentschedules,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ops.gouster.io,resources=environmentschedules/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ops.gouster.io,resources=environmentschedules/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the EnvironmentSchedule object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/reconcile
func (r *EnvironmentScheduleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)

	// Get EnvironmentSchedule instance
	var schedule opsv1alpha1.EnvironmentSchedule
	if err := r.Get(ctx, req.NamespacedName, &schedule); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Get the current hour in the specified time zone
	loc, err := time.LoadLocation(schedule.Spec.TimeZone)
	if err != nil {
		loc = time.UTC
	}
	now := time.Now().In(loc)
	currentHour := now.Hour()

	// Determine if it must be stopped
	shouldBeStopped := false
	if schedule.Spec.StartupHour < schedule.Spec.ShutdownHour {
		if currentHour < schedule.Spec.StartupHour || currentHour >= schedule.Spec.ShutdownHour {
			shouldBeStopped = true
		}
	} else {
		if currentHour < schedule.Spec.StartupHour && currentHour >= schedule.Spec.ShutdownHour {
			shouldBeStopped = true
		}
	}

	// List deployments in the same namespace
	var deploymentsList appsv1.DeploymentList
	if err := r.List(ctx, &deploymentsList, client.InNamespace(req.Namespace)); err != nil {
		logger.Error(err, "Failed to list deployments")
		return ctrl.Result{}, err
	}

	// Scale deployments based on the schedule
	for _, deployment := range deploymentsList.Items {
		if deployment.Labels[CostOptimizerSkipLabel] == "true" {
			logger.Info("Skipping deployment due to cost-optimizer/skip label", "deployment", deployment.Name)
			continue
		}

		if shouldBeStopped {
			if *deployment.Spec.Replicas != 0 {
				// Store the current number of replicas in an annotation before scaling down
				if deployment.Annotations == nil {
					deployment.Annotations = make(map[string]string)
				}
				deployment.Annotations[CostOptimizerPreviousReplicasAnnotation] = strconv.Itoa(int(*deployment.Spec.Replicas))

				// Scale down to 0
				zero := int32(0)
				deployment.Spec.Replicas = &zero
				if err := r.Update(ctx, &deployment); err != nil {
					logger.Error(err, "Failed to scale down deployment", "deployment", deployment.Name)
					continue
				}
				logger.Info("Scaled down deployment", "deployment", deployment.Name)
			}
		} else {
			if *deployment.Spec.Replicas == 0 {
				// Retrieve the previous number of replicas from the annotation
				prevReplicasStr, exists := deployment.Annotations[CostOptimizerPreviousReplicasAnnotation]
				if !exists {
					logger.Info("No previous replicas annotation found, skipping scale up", "deployment", deployment.Name)
					continue
				}

				// Convert the previous replicas string to int32
				var prevReplicas int32
				_, err := fmt.Sscanf(prevReplicasStr, "%d", &prevReplicas)
				if err != nil {
					logger.Error(err, "Failed to parse previous replicas annotation", "deployment", deployment.Name)
					continue
				}

				// Scale up to the previous number of replicas
				deployment.Spec.Replicas = &prevReplicas
				if err := r.Update(ctx, &deployment); err != nil {
					logger.Error(err, "Failed to scale up deployment", "deployment", deployment.Name)
					continue
				}
			}
		}
	}

	// Update the status of the EnvironmentSchedule
	if shouldBeStopped {
		schedule.Status.CurrentStatus = "Stopped"
	} else {
		schedule.Status.CurrentStatus = "Running"
	}
	schedule.Status.LastScheduleTime = now.Format(time.RFC3339)

	if err := r.Status().Update(ctx, &schedule); err != nil {
		logger.Error(err, "Failed to update EnvironmentSchedule status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *EnvironmentScheduleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&opsv1alpha1.EnvironmentSchedule{}).
		Named("environmentschedule").
		Complete(r)
}

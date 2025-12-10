package poolpolicy

import (
	"context"
	"fmt"
	"os"
	"sort"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"

	nodeutil "github.com/llm-d-incubation/llm-d-fast-model-actuation/pkg/utils/node"

	v1alpha1 "github.com/llm-d-incubation/llm-d-fast-model-actuation/api/v1alpha1"
	pkgapi "github.com/llm-d-incubation/llm-d-fast-model-actuation/pkg/api"
)

// Note: LauncherPoolPolicy is namespaced. The controller will read
// LauncherConfig resources and create pods in the same namespace as
// the LauncherPoolPolicy.
type Reconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Logger klog.Logger
}

// Reconcile ensures desired counts of launcher pods per node and per launchConfig.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := klog.FromContext(ctx).WithName("poolpolicy-reconciler")
	var policy v1alpha1.LauncherPoolPolicy
	var reconcileErrors []string
	if err := r.Get(ctx, req.NamespacedName, &policy); err != nil {
		if apierrors.IsNotFound(err) {
			logger.V(4).Info("PoolPolicy not found (deleted)", "name", req.NamespacedName)
			return ctrl.Result{}, nil
		}
		logger.Error(err, "failed to get PoolPolicy", "name", req.NamespacedName)
		return ctrl.Result{}, err
	}

	logger.V(2).Info("Start reconcile", "policy", policy.Name)

	// List all nodes and group those that match each NodePoolSpec
	var nodes corev1.NodeList
	if err := r.List(ctx, &nodes); err != nil {
		logger.Error(err, "failed to list nodes")
		return ctrl.Result{}, err
	}

	// For each NodePoolSpec in the policy, find matching nodes and ensure counts
	for _, np := range policy.Spec.LauncherPoolForNodeType {
		nodePtrs := make([]*corev1.Node, 0, len(nodes.Items))
		for i := range nodes.Items {
			nodePtrs = append(nodePtrs, &nodes.Items[i])
		}
		matchingNodePtrs, err := nodeutil.FilterNodes(nodePtrs, &np.EnhancedNodeSelector)
		if err != nil {
			logger.Error(err, "invalid enhanced node selector", "selector", np.EnhancedNodeSelector)
			continue
		}
		if len(matchingNodePtrs) == 0 {
			logger.V(3).Info("No matching nodes for selector", "selector", np.EnhancedNodeSelector)
			continue
		}

		for _, tmplCount := range np.CountForLauncher {
			tmplName := tmplCount.LauncherConfigName
			if tmplName == "" {
				logger.V(3).Info("Skipping template with empty LauncherConfigName", "policy", policy.Name)
				continue
			}

			// LauncherConfig is expected to live in the same namespace as the policy.
			effectiveNS := policy.Namespace

			var launconfig v1alpha1.LauncherConfig
			if err := r.Get(ctx, apitypes.NamespacedName{Namespace: effectiveNS, Name: tmplName}, &launconfig); err != nil {
				logger.Error(err, "failed to get LauncherConfig", "namespace", effectiveNS, "name", tmplName)
				reconcileErrors = append(reconcileErrors, fmt.Sprintf("failed to get LauncherConfig %s/%s: %v", effectiveNS, tmplName, err))
				// continue to next launcherConfig; if transient error, reconcile will be requeued by the manager
				continue
			}

			// Compute desired launcher pods
			desired := int(tmplCount.LauncherCount)

			for _, nodePtr := range matchingNodePtrs {
				curr, want, err := r.ensureLauncherCount(ctx, &policy, &launconfig, nodePtr, desired, logger)
				if err != nil {
					logger.Error(err, "failed to ensure launcher count", "node", nodePtr.Name, "template", tmplName)
					reconcileErrors = append(reconcileErrors, fmt.Sprintf("node %s template %s: %v", nodePtr.Name, tmplName, err))
				}
				logger.V(2).Info("Node/LauncherConfig counts", "node", nodePtr.Name, "launcherConfig", launconfig.Name, "observed", curr, "desired", want)
			}
		}
	}
	newStatus := policy.Status
	newStatus.ObservedGeneration = int32(policy.Generation)
	newStatus.Errors = reconcileErrors

	if !equalStatus(&policy.Status, &newStatus) {
		policy.Status = newStatus
		if err := r.Status().Update(ctx, &policy); err != nil {
			logger.Error(err, "failed to update LauncherPoolPolicy status", "name", policy.Name)
			return ctrl.Result{}, err
		}
	}

	logger.V(2).Info("Reconcile complete", "policy", policy.Name)
	return ctrl.Result{}, nil
}

// equalStatus helps to compare the relevant status fields.
func equalStatus(a, b *v1alpha1.LauncherPoolPolicyStatus) bool {
	if a.ObservedGeneration != b.ObservedGeneration {
		return false
	}
	if len(a.Errors) != len(b.Errors) {
		return false
	}
	for i := range a.Errors {
		if a.Errors[i] != b.Errors[i] {
			return false
		}
	}
	return true
}

// ensureLauncherCount ensures that exactly desired launcher pods exist on the given node for the given template.
func (r *Reconciler) ensureLauncherCount(ctx context.Context, policy *v1alpha1.LauncherPoolPolicy, launconfig *v1alpha1.LauncherConfig, node *corev1.Node, desired int, logger klog.Logger) (int, int, error) {
	// List pods in policy.Namespace and filter by annotations for this policy & template
	var podList corev1.PodList
	effectiveNS := policy.Namespace
	if err := r.List(ctx, &podList, client.InNamespace(effectiveNS)); err != nil {
		return 0, 0, err
	}

	var existing []*corev1.Pod
	for i := range podList.Items {
		p := &podList.Items[i]
		if p.DeletionTimestamp != nil {
			continue
		}
		if p.Spec.NodeName != node.Name {
			continue
		}
		// Match annotations instead of labels, using keys defined in pkg/api
		ann := p.Annotations
		if ann == nil {
			continue
		}
		if ann[pkgapi.PolicyNameAnnotationName] != policy.Name {
			continue
		}
		if ann[pkgapi.LauncherConfigAnnotationName] != launconfig.Name {
			continue
		}
		if ann[pkgapi.IdleLauncherAnnotationName] != "true" {
			continue
		}
		existing = append(existing, p)
	}

	curr := len(existing)
	logger.V(3).Info("Launcher count", "policy", policy.Name, "template", launconfig.Name, "node", node.Name, "current", curr, "desired", desired)

	if curr < desired {
		toCreate := desired - curr
		for i := 0; i < toCreate; i++ {
			if err := r.createLauncherPod(ctx, policy, launconfig, node, logger); err != nil {
				logger.Error(err, "failed to create launcher pod", "policy", policy.Name, "template", launconfig.Name, "node", node.Name)
				// continue attempting remaining creations
			}
		}
	} else if curr > desired {
		// delete excess pods (delete newest first)
		toDelete := curr - desired
		sort.Slice(existing, func(i, j int) bool {
			return existing[i].CreationTimestamp.After(existing[j].CreationTimestamp.Time)
		})
		for i := 0; i < toDelete && i < len(existing); i++ {
			p := existing[i]
			if err := r.Delete(ctx, p); err != nil && !apierrors.IsNotFound(err) {
				logger.Error(err, "failed to delete excess launcher pod", "pod", p.Name)
			} else {
				logger.V(2).Info("Deleted excess launcher pod", "pod", p.Name)
			}
		}
	}
	return curr, desired, nil
}

// createLauncherPod instantiates a Pod using the LauncherConfig's PodTemplate,
// bound to the given node and owned by the pool policy (TODO: clarify who is owner).
func (r *Reconciler) createLauncherPod(ctx context.Context, policy *v1alpha1.LauncherPoolPolicy, launconfig *v1alpha1.LauncherConfig, node *corev1.Node, logger klog.Logger) error {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: policy.Name + "-" + launconfig.Name + "-" + node.Name + "-",
			Namespace:    policy.Namespace,
			Labels:       map[string]string{},
			Annotations: map[string]string{
				pkgapi.PolicyNameAnnotationName:     policy.Name,
				pkgapi.LauncherConfigAnnotationName: launconfig.Name,
				pkgapi.IdleLauncherAnnotationName:   "true",
				pkgapi.LauncherBasedAnnotationName:  "true",
			},
		},
		Spec: launconfig.Spec.PodTemplate.Spec,
	}

	// Force schedule to specific node
	pod.Spec.NodeName = node.Name

	// Ensure launcher image is set from env var LAUNCHER_IMAGE or constructed from
	// CONTAINER_IMG_REG / LAUNCHER_IMG_REPO / LAUNCHER_IMG_TAG. If none available,
	// leave whatever image is already present in the pod template.
	launcherImage := os.Getenv("LAUNCHER_IMAGE")
	if launcherImage == "" {
		reg := os.Getenv("CONTAINER_IMG_REG")
		repo := os.Getenv("LAUNCHER_IMG_REPO")
		tag := os.Getenv("LAUNCHER_IMG_TAG")
		if reg != "" && repo != "" && tag != "" {
			launcherImage = fmt.Sprintf("%s/%s:%s", reg, repo, tag)
		}
	}
	if launcherImage != "" {
		if len(pod.Spec.Containers) > 0 {
			for i := range pod.Spec.Containers {
				pod.Spec.Containers[i].Image = launcherImage
			}
		} else {
			pod.Spec.Containers = []corev1.Container{{Name: "launcher", Image: launcherImage}}
		}
		logger.V(3).Info("Set launcher pod image", "image", launcherImage)
	}

	// Ensure restart policy
	if pod.Spec.RestartPolicy == "" {
		pod.Spec.RestartPolicy = corev1.RestartPolicyAlways
	}

	// Set owner reference to the policy so pods are garbage-collected if the policy is removed
	if err := controllerutil.SetOwnerReference(policy, pod, r.Scheme); err != nil {
		logger.Error(err, "failed to set owner reference on pod")
	}

	return r.Create(ctx, pod)
}

// SetupWithManager registers this controller with the provided manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Scheme != nil {
		if err := v1alpha1.AddToScheme(r.Scheme); err != nil {
			return err
		}
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.LauncherPoolPolicy{}).
		Complete(r)
}

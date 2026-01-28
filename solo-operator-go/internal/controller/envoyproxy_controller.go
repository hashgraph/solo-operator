// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

const (
	// EnvoyProxyFinalizer is the finalizer for EnvoyProxy resources
	EnvoyProxyFinalizer = "operator.solo.hedera.com/envoyproxy-finalizer"

	// DefaultEnvoyImage is the default Envoy image
	DefaultEnvoyImage = "envoyproxy/envoy:v1.21.6"
)

// EnvoyProxyReconciler reconciles an EnvoyProxy object
type EnvoyProxyReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=envoyproxies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=envoyproxies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.solo.hedera.com,resources=envoyproxies/finalizers,verbs=update

// Reconcile is the main reconciliation loop for EnvoyProxy resources
func (r *EnvoyProxyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("Starting EnvoyProxy reconciliation",
		"namespace", req.Namespace,
		"name", req.Name)
	defer logger.Info("Finished EnvoyProxy reconciliation",
		"namespace", req.Namespace,
		"name", req.Name)

	// Fetch the EnvoyProxy instance
	envoy := &operatorv1alpha1.EnvoyProxy{}
	if err := r.Get(ctx, req.NamespacedName, envoy); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("EnvoyProxy resource not found, ignoring since it must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get EnvoyProxy")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !envoy.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, envoy)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(envoy, EnvoyProxyFinalizer) {
		controllerutil.AddFinalizer(envoy, EnvoyProxyFinalizer)
		if err := r.Update(ctx, envoy); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Reconcile ConfigMap
	if err := r.reconcileConfigMap(ctx, envoy); err != nil {
		return ctrl.Result{}, err
	}

	// Reconcile Deployment
	if err := r.reconcileDeployment(ctx, envoy); err != nil {
		return ctrl.Result{}, err
	}

	// Reconcile Service
	if err := r.reconcileService(ctx, envoy); err != nil {
		return ctrl.Result{}, err
	}

	// Update status
	if err := r.updateStatus(ctx, envoy); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// handleDeletion handles the deletion of an EnvoyProxy resource
func (r *EnvoyProxyReconciler) handleDeletion(ctx context.Context, envoy *operatorv1alpha1.EnvoyProxy) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if controllerutil.ContainsFinalizer(envoy, EnvoyProxyFinalizer) {
		logger.Info("Deleting EnvoyProxy",
			"namespace", envoy.Namespace,
			"name", envoy.Name)

		// Remove finalizer
		controllerutil.RemoveFinalizer(envoy, EnvoyProxyFinalizer)
		if err := r.Update(ctx, envoy); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// reconcileConfigMap ensures the Envoy config map exists
func (r *EnvoyProxyReconciler) reconcileConfigMap(ctx context.Context, envoy *operatorv1alpha1.EnvoyProxy) error {
	logger := log.FromContext(ctx)

	cmName := fmt.Sprintf("%s-config", envoy.Name)

	// Use provided files or default config
	files := envoy.Spec.Files
	if files == nil || len(files) == 0 {
		files = r.defaultEnvoyConfig(envoy)
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmName,
			Namespace: envoy.Namespace,
			Labels:    r.commonLabels(envoy),
		},
		Data: files,
	}

	if err := controllerutil.SetControllerReference(envoy, cm, r.Scheme); err != nil {
		return err
	}

	existing := &corev1.ConfigMap{}
	err := r.Get(ctx, client.ObjectKey{Name: cm.Name, Namespace: cm.Namespace}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Creating Envoy config map", "name", cm.Name)
			return r.Create(ctx, cm)
		}
		return err
	}

	existing.Data = cm.Data
	return r.Update(ctx, existing)
}

// defaultEnvoyConfig returns the default Envoy configuration
func (r *EnvoyProxyReconciler) defaultEnvoyConfig(envoy *operatorv1alpha1.EnvoyProxy) map[string]string {
	proxyPort := envoy.Spec.ProxyPort
	if proxyPort == 0 {
		proxyPort = 8081
	}

	config := fmt.Sprintf(`admin:
  address:
    pipe:
      path: /admin/admin.sock
static_resources:
  listeners:
    - name: listener_0
      address:
        socket_address:
          address: 0.0.0.0
          port_value: %d
      filter_chains:
        - filters:
            - name: envoy.filters.network.http_connection_manager
              typed_config:
                '@type': type.googleapis.com/envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager
                codec_type: auto
                stat_prefix: ingress_http
                use_remote_address: true
                skip_xff_append: false
                route_config:
                  name: local_route
                  virtual_hosts:
                    - name: local_service
                      domains:
                        - '*'
                      routes:
                        - match:
                            prefix: /
                          route:
                            cluster: echo_service
                            timeout: 0s
                            max_stream_duration:
                              grpc_timeout_header_max: 0s
                      cors:
                        allow_origin_string_match:
                          - prefix: '*'
                        allow_methods: GET, PUT, DELETE, POST, OPTIONS
                        allow_headers: keep-alive,user-agent,cache-control,content-type,content-transfer-encoding,custom-header-1,x-accept-content-transfer-encoding,x-accept-response-streaming,x-user-agent,x-grpc-web,grpc-timeout
                        max_age: "1728000"
                        expose_headers: custom-header-1,grpc-status,grpc-message
                http_filters:
                  - name: envoy.filters.http.grpc_web
                  - name: envoy.filters.http.cors
                  - name: envoy.filters.http.router
  clusters:
    - name: echo_service
      connect_timeout: 0.25s
      type: logical_dns
      http2_protocol_options: {}
      lb_policy: round_robin
      load_assignment:
        cluster_name: cluster_0
        endpoints:
          - lb_endpoints:
              - endpoint:
                  address:
                    socket_address:
                      address: hedera-proxy
                      port_value: 50211
`, proxyPort)

	return map[string]string{
		"envoy.yaml": config,
	}
}

// reconcileDeployment ensures the Envoy deployment exists
func (r *EnvoyProxyReconciler) reconcileDeployment(ctx context.Context, envoy *operatorv1alpha1.EnvoyProxy) error {
	logger := log.FromContext(ctx)

	replicas := int32(2)
	if envoy.Spec.Replicas != nil {
		replicas = *envoy.Spec.Replicas
	}

	labels := r.commonLabels(envoy)
	cmName := fmt.Sprintf("%s-config", envoy.Name)

	image := envoy.Spec.Image
	if image == "" {
		image = DefaultEnvoyImage
	}

	proxyPort := envoy.Spec.ProxyPort
	if proxyPort == 0 {
		proxyPort = 8081
	}

	adminPort := envoy.Spec.AdminPort
	if adminPort == 0 {
		adminPort = 9901
	}

	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      envoy.Name,
			Namespace: envoy.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "envoy",
							Image: image,
							Args:  []string{"-c", "/etc/envoy/envoy.yaml"},
							Ports: []corev1.ContainerPort{
								{Name: "http", ContainerPort: proxyPort, Protocol: corev1.ProtocolTCP},
								{Name: "admin", ContainerPort: adminPort, Protocol: corev1.ProtocolTCP},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "config",
									MountPath: "/etc/envoy",
									ReadOnly:  true,
								},
								{
									Name:      "admin",
									MountPath: "/admin",
								},
							},
							Env: r.buildEnv(envoy),
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/ready",
										Port: intstr.FromInt32(adminPort),
									},
								},
								InitialDelaySeconds: 10,
								PeriodSeconds:       10,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/ready",
										Port: intstr.FromInt32(adminPort),
									},
								},
								InitialDelaySeconds: 5,
								PeriodSeconds:       5,
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{Name: cmName},
								},
							},
						},
						{
							Name: "admin",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
		},
	}

	if err := controllerutil.SetControllerReference(envoy, deploy, r.Scheme); err != nil {
		return err
	}

	existing := &appsv1.Deployment{}
	err := r.Get(ctx, client.ObjectKey{Name: deploy.Name, Namespace: deploy.Namespace}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Creating Envoy deployment", "name", deploy.Name)
			return r.Create(ctx, deploy)
		}
		return err
	}

	existing.Spec = deploy.Spec
	return r.Update(ctx, existing)
}

// buildEnv builds the environment variables for the Envoy container
func (r *EnvoyProxyReconciler) buildEnv(envoy *operatorv1alpha1.EnvoyProxy) []corev1.EnvVar {
	if len(envoy.Spec.Env) > 0 {
		return envoy.Spec.Env
	}

	// Default environment variables
	return []corev1.EnvVar{
		{
			Name: "SERVICE_CLUSTER",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.namespace",
				},
			},
		},
		{
			Name: "SERVICE_NODE",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		},
	}
}

// reconcileService ensures the Envoy service exists
func (r *EnvoyProxyReconciler) reconcileService(ctx context.Context, envoy *operatorv1alpha1.EnvoyProxy) error {
	logger := log.FromContext(ctx)

	proxyPort := envoy.Spec.ProxyPort
	if proxyPort == 0 {
		proxyPort = 8081
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      envoy.Name,
			Namespace: envoy.Namespace,
			Labels:    r.commonLabels(envoy),
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: r.commonLabels(envoy),
			Ports: []corev1.ServicePort{
				{Name: "http", Port: proxyPort, TargetPort: intstr.FromInt32(proxyPort), Protocol: corev1.ProtocolTCP},
			},
		},
	}

	if err := controllerutil.SetControllerReference(envoy, svc, r.Scheme); err != nil {
		return err
	}

	existing := &corev1.Service{}
	err := r.Get(ctx, client.ObjectKey{Name: svc.Name, Namespace: svc.Namespace}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Creating Envoy service", "name", svc.Name)
			return r.Create(ctx, svc)
		}
		return err
	}

	existing.Spec.Selector = svc.Spec.Selector
	existing.Spec.Ports = svc.Spec.Ports
	return r.Update(ctx, existing)
}

// updateStatus updates the EnvoyProxy status
func (r *EnvoyProxyReconciler) updateStatus(ctx context.Context, envoy *operatorv1alpha1.EnvoyProxy) error {
	// Get deployment status
	deploy := &appsv1.Deployment{}
	if err := r.Get(ctx, client.ObjectKey{Name: envoy.Name, Namespace: envoy.Namespace}, deploy); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		// Deployment not found
		envoy.Status.Replicas = 0
		envoy.Status.ReadyReplicas = 0
		envoy.Status.AvailableReplicas = 0
		envoy.Status.UnavailableReplicas = 0
		envoy.Status.UpdatedReplicas = 0
	} else {
		envoy.Status.Replicas = deploy.Status.Replicas
		envoy.Status.ReadyReplicas = deploy.Status.ReadyReplicas
		envoy.Status.AvailableReplicas = deploy.Status.AvailableReplicas
		envoy.Status.UnavailableReplicas = deploy.Status.UnavailableReplicas
		envoy.Status.UpdatedReplicas = deploy.Status.UpdatedReplicas
		envoy.Status.Conditions = deploy.Status.Conditions
	}

	envoy.Status.ObservedGeneration = envoy.Generation

	return r.Status().Update(ctx, envoy)
}

// commonLabels returns the common labels for resources
func (r *EnvoyProxyReconciler) commonLabels(envoy *operatorv1alpha1.EnvoyProxy) map[string]string {
	return map[string]string{
		LabelManagedBy: "solo-operator",
		LabelPartOf:    "envoy-proxy",
		"app":          envoy.Name,
	}
}

// SetupWithManager sets up the controller with the Manager
func (r *EnvoyProxyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.EnvoyProxy{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Service{}).
		Owns(&appsv1.Deployment{}).
		Named("envoyproxy").
		Complete(r)
}

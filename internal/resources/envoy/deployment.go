// SPDX-License-Identifier: Apache-2.0

package envoy

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strconv"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
	"github.com/hashgraph/solo-operator/internal/subreconciler"
)

type DeploymentSubReconciler struct {
	*subreconciler.BaseSubReconciler[*operatorv1alpha1.EnvoyProxy]
}

func NewDeploymentSubReconciler(c client.Client, scheme *runtime.Scheme) *DeploymentSubReconciler {
	return &DeploymentSubReconciler{
		BaseSubReconciler: subreconciler.NewBaseSubReconciler[*operatorv1alpha1.EnvoyProxy](c, scheme, "Deployment"),
	}
}

func (r *DeploymentSubReconciler) Reconcile(ctx context.Context, envoy *operatorv1alpha1.EnvoyProxy) (ctrl.Result, error) {
	desired, err := r.buildDeployment(envoy)
	if err != nil {
		return ctrl.Result{}, err
	}

	if err := controllerutil.SetControllerReference(envoy, desired, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	existing := &appsv1.Deployment{}
	err = r.Client.Get(ctx, client.ObjectKey{Name: desired.Name, Namespace: desired.Namespace}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			common.Info(ctx, common.ReasonDeploymentCreated).Msg("Creating Envoy deployment")
			if err := r.Client.Create(ctx, desired); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if r.needsUpdate(existing, desired) {
		common.Debug(ctx, common.ReasonDeploymentUpdated).Msg("Updating Envoy deployment")
		existing.Spec = desired.Spec
		existing.Labels = desired.Labels
		return ctrl.Result{}, common.WarnOnConflict(ctx, r.Client.Update(ctx, existing))
	}

	return ctrl.Result{}, nil
}

var (
	defaultStartupProbe   = corev1.Probe{InitialDelaySeconds: 5, PeriodSeconds: 5, TimeoutSeconds: 1, FailureThreshold: 12, SuccessThreshold: 1}
	defaultLivenessProbe  = corev1.Probe{PeriodSeconds: 10, TimeoutSeconds: 1, FailureThreshold: 3, SuccessThreshold: 1}
	defaultReadinessProbe = corev1.Probe{InitialDelaySeconds: 5, PeriodSeconds: 5, TimeoutSeconds: 1, FailureThreshold: 3, SuccessThreshold: 1}
)

func ProbesEnabled(envoy *operatorv1alpha1.EnvoyProxy) bool {
	p := envoy.Spec.Probes
	return p == nil || p.Enabled == nil || *p.Enabled
}

func buildProbe(handler corev1.ProbeHandler, spec *operatorv1alpha1.EnvoyProbeSpec, defaults corev1.Probe) *corev1.Probe {
	probe := defaults
	probe.ProbeHandler = handler
	if spec == nil {
		return &probe
	}
	if spec.InitialDelaySeconds != nil {
		probe.InitialDelaySeconds = *spec.InitialDelaySeconds
	}
	if spec.PeriodSeconds != nil {
		probe.PeriodSeconds = *spec.PeriodSeconds
	}
	if spec.TimeoutSeconds != nil {
		probe.TimeoutSeconds = *spec.TimeoutSeconds
	}
	if spec.FailureThreshold != nil {
		probe.FailureThreshold = *spec.FailureThreshold
	}
	return &probe
}

func (r *DeploymentSubReconciler) buildDeployment(envoy *operatorv1alpha1.EnvoyProxy) (*appsv1.Deployment, error) {
	checksum, err := restartChecksum(envoy)
	if err != nil {
		return nil, err
	}

	replicas := GetReplicas(envoy)
	resourceLabels := ResourceLabels(envoy)
	listeners := ResolveListeners(envoy)

	gracePeriod := envoy.Spec.TerminationGracePeriodSeconds
	if gracePeriod == 0 {
		gracePeriod = 10
	}

	container := corev1.Container{
		Name:            ContainerName,
		Image:           GetImage(envoy),
		Args:            []string{"-c", ConfigMountPath + "/" + ConfigFileName, "--drain-time-s", strconv.FormatInt(GetDrainSeconds(envoy), 10)},
		Ports:           buildContainerPorts(envoy, listeners),
		Env:             envoy.Spec.Env,
		SecurityContext: common.RestrictedSecurityContext(),
	}
	if envoy.Spec.Resources != nil {
		container.Resources = *envoy.Spec.Resources
	}
	volumes := []corev1.Volume{{
		Name: ConfigVolumeName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: ConfigMapName(envoy)}},
		},
	}}
	container.VolumeMounts = []corev1.VolumeMount{{Name: ConfigVolumeName, MountPath: ConfigMountPath, ReadOnly: true}}
	for _, l := range listeners {
		if l.TLSSecretName == "" {
			continue
		}
		volumes = append(volumes, corev1.Volume{
			Name: l.TLSVolume(),
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: l.TLSSecretName,
					Items:      []corev1.KeyToPath{{Key: "tls.crt", Path: "tls.crt"}, {Key: "tls.key", Path: "tls.key"}},
				},
			},
		})
		container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{Name: l.TLSVolume(), MountPath: l.TLSDir(), ReadOnly: true})
	}

	// spec.files may not expose /healthz or the listeners, so probes are omitted then.
	if len(envoy.Spec.Files) == 0 && ProbesEnabled(envoy) {
		probes := envoy.Spec.Probes
		if probes == nil {
			probes = &operatorv1alpha1.EnvoyProbesSpec{}
		}
		healthz := corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{Path: "/healthz", Port: intstr.FromInt32(GetMetricsPort(envoy)), Scheme: corev1.URISchemeHTTP}}
		container.StartupProbe = buildProbe(healthz, probes.Startup, defaultStartupProbe)
		container.LivenessProbe = buildProbe(healthz, probes.Liveness, defaultLivenessProbe)
		// A rejected LDS leaves the traffic ports unbound while /healthz still answers.
		ready := healthz
		if len(listeners) > 0 {
			ready = corev1.ProbeHandler{TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromInt32(listeners[0].Port)}}
		}
		container.ReadinessProbe = buildProbe(ready, probes.Readiness, defaultReadinessProbe)
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      envoy.Name,
			Namespace: envoy.Namespace,
			Labels:    resourceLabels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: CommonLabels(envoy)},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      resourceLabels,
					Annotations: map[string]string{ConfigChecksumAnnotation: checksum},
				},
				Spec: corev1.PodSpec{
					NodeSelector:                  envoy.Spec.NodeSelector,
					Affinity:                      envoy.Spec.Affinity,
					Tolerations:                   envoy.Spec.Tolerations,
					TerminationGracePeriodSeconds: &gracePeriod,
					Containers:                    []corev1.Container{container},
					Volumes:                       volumes,
				},
			},
		},
	}, nil
}

func restartChecksum(envoy *operatorv1alpha1.EnvoyProxy) (string, error) {
	bootstrap, ok := envoy.Spec.Files[ConfigFileName]
	if !ok {
		var err error
		if bootstrap, err = BootstrapConfig(envoy); err != nil {
			return "", err
		}
	}
	return fmt.Sprintf("%x", sha256.Sum256([]byte(bootstrap))), nil
}

func buildContainerPorts(envoy *operatorv1alpha1.EnvoyProxy, listeners []Listener) []corev1.ContainerPort {
	ports := []corev1.ContainerPort{{Name: "metrics", ContainerPort: GetMetricsPort(envoy), Protocol: corev1.ProtocolTCP}}
	for _, l := range listeners {
		ports = append(ports, corev1.ContainerPort{Name: l.PortName, ContainerPort: l.Port, Protocol: corev1.ProtocolTCP})
	}
	if envoy.Spec.ExposeAdmin {
		ports = append(ports, corev1.ContainerPort{Name: "admin", ContainerPort: GetAdminPort(envoy), Protocol: corev1.ProtocolTCP})
	}
	return ports
}

func (r *DeploymentSubReconciler) needsUpdate(existing, desired *appsv1.Deployment) bool {
	if existing.Spec.Replicas == nil || *existing.Spec.Replicas != *desired.Spec.Replicas {
		return true
	}

	ep := existing.Spec.Template.Spec
	dp := desired.Spec.Template.Spec

	if !equality.Semantic.DeepEqual(ep.NodeSelector, dp.NodeSelector) {
		return true
	}
	if !equality.Semantic.DeepEqual(ep.Affinity, dp.Affinity) {
		return true
	}
	if !equality.Semantic.DeepEqual(ep.Tolerations, dp.Tolerations) {
		return true
	}
	if !equality.Semantic.DeepEqual(ep.TerminationGracePeriodSeconds, dp.TerminationGracePeriodSeconds) {
		return true
	}
	if !equality.Semantic.DeepEqual(ep.Volumes, dp.Volumes) {
		return true
	}

	if len(ep.Containers) != len(dp.Containers) {
		return true
	}
	for i := range dp.Containers {
		ec := ep.Containers[i]
		dc := dp.Containers[i]
		if ec.Name != dc.Name || ec.Image != dc.Image {
			return true
		}
		if !equality.Semantic.DeepEqual(ec.Args, dc.Args) || !equality.Semantic.DeepEqual(ec.Resources, dc.Resources) {
			return true
		}
		if !equality.Semantic.DeepEqual(ec.Ports, dc.Ports) {
			return true
		}
		if !equality.Semantic.DeepEqual(ec.Env, dc.Env) {
			return true
		}
		if !equality.Semantic.DeepEqual(ec.VolumeMounts, dc.VolumeMounts) {
			return true
		}
		if !equality.Semantic.DeepEqual(ec.StartupProbe, dc.StartupProbe) {
			return true
		}
		if !equality.Semantic.DeepEqual(ec.LivenessProbe, dc.LivenessProbe) {
			return true
		}
		if !equality.Semantic.DeepEqual(ec.ReadinessProbe, dc.ReadinessProbe) {
			return true
		}
	}

	// Check pod template labels bidirectionally (captures AdditionalLabels additions and removals)
	if len(existing.Spec.Template.Labels) != len(desired.Spec.Template.Labels) {
		return true
	}
	for k, v := range desired.Spec.Template.Labels {
		if existing.Spec.Template.Labels[k] != v {
			return true
		}
	}

	return existing.Spec.Template.Annotations[ConfigChecksumAnnotation] != desired.Spec.Template.Annotations[ConfigChecksumAnnotation]
}

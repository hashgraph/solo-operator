// SPDX-License-Identifier: Apache-2.0

package envoy

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strconv"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
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

	// No DeepEqual guard: server-side defaulting makes local diffs phantom (see consensus StatefulSet, #1219).
	existing.Spec = desired.Spec
	existing.Labels = desired.Labels
	return ctrl.Result{}, common.WarnOnConflict(ctx, r.Client.Update(ctx, existing))
}

var (
	defaultStartupProbe   = corev1.Probe{InitialDelaySeconds: 5, PeriodSeconds: 5, TimeoutSeconds: 1, FailureThreshold: 12}
	defaultLivenessProbe  = corev1.Probe{PeriodSeconds: 10, TimeoutSeconds: 1, FailureThreshold: 3}
	defaultReadinessProbe = corev1.Probe{InitialDelaySeconds: 5, PeriodSeconds: 5, TimeoutSeconds: 1, FailureThreshold: 3}
)

func buildProbe(handler corev1.ProbeHandler, spec *operatorv1alpha1.EnvoyProbeSpec, defaults corev1.Probe) *corev1.Probe {
	probe := defaults
	probe.ProbeHandler = handler
	if spec != nil {
		probe.InitialDelaySeconds = ptr.Deref(spec.InitialDelaySeconds, probe.InitialDelaySeconds)
		probe.PeriodSeconds = ptr.Deref(spec.PeriodSeconds, probe.PeriodSeconds)
		probe.TimeoutSeconds = ptr.Deref(spec.TimeoutSeconds, probe.TimeoutSeconds)
		probe.FailureThreshold = ptr.Deref(spec.FailureThreshold, probe.FailureThreshold)
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
	probes := ptr.Deref(envoy.Spec.Probes, operatorv1alpha1.EnvoyProbesSpec{})
	if len(envoy.Spec.Files) == 0 && ptr.Deref(probes.Enabled, true) {
		healthz := corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{Path: "/healthz", Port: intstr.FromInt32(GetMetricsPort(envoy))}}
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

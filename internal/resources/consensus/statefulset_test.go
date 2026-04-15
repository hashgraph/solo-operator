// SPDX-License-Identifier: Apache-2.0

package consensus

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

func newStsTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(s)
	_ = operatorv1alpha1.AddToScheme(s)
	return s
}

func newStsReconciler() *StatefulSetSubReconciler {
	scheme := newStsTestScheme()
	c := fake.NewClientBuilder().WithScheme(scheme).Build()
	return NewStatefulSetSubReconciler(c, scheme)
}

func capsuleWithImage(image string) *operatorv1alpha1.ConsensusCapsule {
	return &operatorv1alpha1.ConsensusCapsule{
		ObjectMeta: metav1.ObjectMeta{Name: "node0", Namespace: "test"},
		Spec: operatorv1alpha1.ConsensusCapsuleSpec{
			Orbit:  "test-orbit",
			NodeId: 0,
			PodProperties: operatorv1alpha1.ConsensusPodProperties{
				Containers: operatorv1alpha1.ConsensusContainers{
					ConsensusNode: operatorv1alpha1.ConsensusNodeContainer{
						SoftwareVersion: &operatorv1alpha1.SoftwareVersion{
							Repository: "hashgraph",
							ImageName:  "hedera-node",
							ImageTag:   image,
						},
					},
				},
			},
		},
	}
}

// ── buildStatefulSet ──────────────────────────────────────────────────────────

func TestBuildStatefulSet_GenesisInitContainerPresent(t *testing.T) {
	r := newStsReconciler()
	capsule := capsuleWithImage("v0.50.0")

	sts, err := r.buildStatefulSet(context.Background(), capsule)
	if err != nil {
		t.Fatalf("buildStatefulSet: %v", err)
	}

	if len(sts.Spec.Template.Spec.InitContainers) != 1 {
		t.Fatalf("expected 1 init container, got %d", len(sts.Spec.Template.Spec.InitContainers))
	}
	ic := sts.Spec.Template.Spec.InitContainers[0]
	if ic.Name != "genesis-init" {
		t.Errorf("expected init container named genesis-init, got %q", ic.Name)
	}
	if ic.Image != GenesisInitImage {
		t.Errorf("expected init container image %q, got %q", GenesisInitImage, ic.Image)
	}
}

func TestBuildStatefulSet_JavaEnvVars(t *testing.T) {
	r := newStsReconciler()
	capsule := capsuleWithImage("v0.50.0")
	capsule.Spec.PodProperties.Containers.ConsensusNode.JavaHeapMin = "4g"
	capsule.Spec.PodProperties.Containers.ConsensusNode.JavaHeapMax = "8g"
	capsule.Spec.PodProperties.Containers.ConsensusNode.JavaOpts = "-XX:+UseG1GC"

	sts, err := r.buildStatefulSet(context.Background(), capsule)
	if err != nil {
		t.Fatalf("buildStatefulSet: %v", err)
	}

	envMap := make(map[string]string)
	for _, e := range sts.Spec.Template.Spec.Containers[0].Env {
		envMap[e.Name] = e.Value
	}

	checks := map[string]string{
		"JAVA_HEAP_MIN": "4g",
		"JAVA_HEAP_MAX": "8g",
		"JAVA_OPTS":     "-XX:+UseG1GC",
	}
	for k, want := range checks {
		if got := envMap[k]; got != want {
			t.Errorf("env %s: want %q, got %q", k, want, got)
		}
	}
}

func TestBuildStatefulSet_JavaEnvVars_NotSetWhenEmpty(t *testing.T) {
	r := newStsReconciler()
	capsule := capsuleWithImage("v0.50.0")
	// JavaHeapMin/Max/Opts left unset

	sts, err := r.buildStatefulSet(context.Background(), capsule)
	if err != nil {
		t.Fatalf("buildStatefulSet: %v", err)
	}

	for _, e := range sts.Spec.Template.Spec.Containers[0].Env {
		if e.Name == "JAVA_HEAP_MIN" || e.Name == "JAVA_HEAP_MAX" || e.Name == "JAVA_OPTS" {
			t.Errorf("unexpected env var %q when field is empty", e.Name)
		}
	}
}

func TestBuildStatefulSet_HapiAppSecretsMount(t *testing.T) {
	r := newStsReconciler()
	capsule := capsuleWithImage("v0.50.0")
	capsule.Spec.HapiAppSecretsName = "my-tls-secret"

	sts, err := r.buildStatefulSet(context.Background(), capsule)
	if err != nil {
		t.Fatalf("buildStatefulSet: %v", err)
	}

	found := false
	for _, v := range sts.Spec.Template.Spec.Volumes {
		if v.Name == "hapi-app-secrets" && v.Secret != nil && v.Secret.SecretName == "my-tls-secret" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected hapi-app-secrets volume backed by my-tls-secret")
	}

	certMount, keyMount := false, false
	for _, m := range sts.Spec.Template.Spec.Containers[0].VolumeMounts {
		if m.Name == "hapi-app-secrets" && m.SubPath == "hedera.crt" {
			certMount = true
		}
		if m.Name == "hapi-app-secrets" && m.SubPath == "hedera.key" {
			keyMount = true
		}
	}
	if !certMount {
		t.Error("expected hedera.crt volume mount")
	}
	if !keyMount {
		t.Error("expected hedera.key volume mount")
	}
}

func TestBuildStatefulSet_BlockNodesMount(t *testing.T) {
	r := newStsReconciler()
	capsule := capsuleWithImage("v0.50.0")
	capsule.Spec.BlockNodes = &operatorv1alpha1.BlockNodesConfig{} // non-nil triggers the configmap volume

	sts, err := r.buildStatefulSet(context.Background(), capsule)
	if err != nil {
		t.Fatalf("buildStatefulSet: %v", err)
	}

	found := false
	for _, v := range sts.Spec.Template.Spec.Volumes {
		if v.Name == "block-nodes-config" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected block-nodes-config volume when BlockNodes is set")
	}
}

func TestBuildStatefulSet_ServiceName(t *testing.T) {
	r := newStsReconciler()
	capsule := capsuleWithImage("v0.50.0")

	sts, err := r.buildStatefulSet(context.Background(), capsule)
	if err != nil {
		t.Fatalf("buildStatefulSet: %v", err)
	}

	if sts.Spec.ServiceName != ConsensusSubdomain {
		t.Errorf("expected ServiceName %q, got %q", ConsensusSubdomain, sts.Spec.ServiceName)
	}
}

// ── needsUpdate ───────────────────────────────────────────────────────────────

func makeSts(image string, envVars []corev1.EnvVar) *appsv1.StatefulSet {
	replicas := int32(1)
	return &appsv1.StatefulSet{
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{
						{Name: "genesis-init", Image: GenesisInitImage},
					},
					Containers: []corev1.Container{
						{Name: "consensus-node", Image: image, Env: envVars},
					},
					Volumes: []corev1.Volume{
						{Name: "hapi-app-config"},
						{Name: "data"},
					},
				},
			},
		},
	}
}

func TestNeedsUpdate_NoChange(t *testing.T) {
	r := NewStatefulSetSubReconciler(fake.NewClientBuilder().Build(), runtime.NewScheme())
	sts := makeSts("v0.50.0", nil)
	if r.needsUpdate(sts, sts) {
		t.Error("identical StatefulSets should not need update")
	}
}

func TestNeedsUpdate_ImageChange(t *testing.T) {
	r := NewStatefulSetSubReconciler(fake.NewClientBuilder().Build(), runtime.NewScheme())
	existing := makeSts("v0.50.0", nil)
	desired := makeSts("v0.51.0", nil)
	if !r.needsUpdate(existing, desired) {
		t.Error("image change should trigger update")
	}
}

func TestNeedsUpdate_EnvVarAdded(t *testing.T) {
	r := NewStatefulSetSubReconciler(fake.NewClientBuilder().Build(), runtime.NewScheme())
	existing := makeSts("v0.50.0", nil)
	desired := makeSts("v0.50.0", []corev1.EnvVar{{Name: "JAVA_HEAP_MIN", Value: "4g"}})
	if !r.needsUpdate(existing, desired) {
		t.Error("adding an env var should trigger update")
	}
}

func TestNeedsUpdate_EnvVarValueChanged(t *testing.T) {
	r := NewStatefulSetSubReconciler(fake.NewClientBuilder().Build(), runtime.NewScheme())
	existing := makeSts("v0.50.0", []corev1.EnvVar{{Name: "JAVA_HEAP_MIN", Value: "4g"}})
	desired := makeSts("v0.50.0", []corev1.EnvVar{{Name: "JAVA_HEAP_MIN", Value: "8g"}})
	if !r.needsUpdate(existing, desired) {
		t.Error("env var value change should trigger update")
	}
}

func TestNeedsUpdate_InitContainerImageChange(t *testing.T) {
	r := NewStatefulSetSubReconciler(fake.NewClientBuilder().Build(), runtime.NewScheme())
	existing := makeSts("v0.50.0", nil)
	desired := makeSts("v0.50.0", nil)
	desired.Spec.Template.Spec.InitContainers[0].Image = "busybox:1.37.0"
	if !r.needsUpdate(existing, desired) {
		t.Error("init container image change should trigger update")
	}
}

func TestNeedsUpdate_VolumeAdded(t *testing.T) {
	r := NewStatefulSetSubReconciler(fake.NewClientBuilder().Build(), runtime.NewScheme())
	existing := makeSts("v0.50.0", nil)
	desired := makeSts("v0.50.0", nil)
	desired.Spec.Template.Spec.Volumes = append(desired.Spec.Template.Spec.Volumes,
		corev1.Volume{Name: "new-secret-vol"})
	if !r.needsUpdate(existing, desired) {
		t.Error("adding a volume should trigger update")
	}
}

func TestNeedsUpdate_VolumeSourceChanged(t *testing.T) {
	r := NewStatefulSetSubReconciler(fake.NewClientBuilder().Build(), runtime.NewScheme())
	existing := makeSts("v0.50.0", nil)
	desired := makeSts("v0.50.0", nil)
	// Same volume name, different ConfigMap backing — old code missed this.
	desired.Spec.Template.Spec.Volumes[0] = corev1.Volume{
		Name: existing.Spec.Template.Spec.Volumes[0].Name,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: "new-cm"},
			},
		},
	}
	if !r.needsUpdate(existing, desired) {
		t.Error("volume source change should trigger update")
	}
}

func TestNeedsUpdate_EnvValueFromChanged(t *testing.T) {
	r := NewStatefulSetSubReconciler(fake.NewClientBuilder().Build(), runtime.NewScheme())
	existing := makeSts("v0.50.0", []corev1.EnvVar{
		{
			Name: "POD_IP",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{FieldPath: "status.podIP"},
			},
		},
	})
	desired := makeSts("v0.50.0", []corev1.EnvVar{
		{
			Name: "POD_IP",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{FieldPath: "status.hostIP"},
			},
		},
	})
	if !r.needsUpdate(existing, desired) {
		t.Error("ValueFrom FieldRef change should trigger update")
	}
}

func TestNeedsUpdate_InitContainerCommandChanged(t *testing.T) {
	r := NewStatefulSetSubReconciler(fake.NewClientBuilder().Build(), runtime.NewScheme())
	existing := makeSts("v0.50.0", nil)
	desired := makeSts("v0.50.0", nil)
	desired.Spec.Template.Spec.InitContainers[0].Command = []string{"sh", "-c", "echo updated"}
	if !r.needsUpdate(existing, desired) {
		t.Error("init container command change should trigger update")
	}
}

func TestNeedsUpdate_ContainerSecurityContextChanged(t *testing.T) {
	r := NewStatefulSetSubReconciler(fake.NewClientBuilder().Build(), runtime.NewScheme())
	existing := makeSts("v0.50.0", nil)
	desired := makeSts("v0.50.0", nil)
	privileged := true
	desired.Spec.Template.Spec.Containers[0].SecurityContext = &corev1.SecurityContext{
		Privileged: &privileged,
	}
	if !r.needsUpdate(existing, desired) {
		t.Error("container SecurityContext change should trigger update")
	}
}

// ── pod-level volume deduplication ───────────────────────────────────────────

func TestBuildVolumes_PodVolumeOverrideReplacesExisting(t *testing.T) {
	r := NewStatefulSetSubReconciler(fake.NewClientBuilder().Build(), runtime.NewScheme())
	capsule := capsuleWithImage("v0.50.0")

	// Override the "saved" volume (which is always built by the operator) with a custom one.
	customSaved := corev1.Volume{
		Name: VolumeTypeSaved,
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{Path: "/mnt/custom-saved"},
		},
	}
	capsule.Spec.PodProperties.Volumes = &operatorv1alpha1.ConsensusPodVolumes{
		Saved: &customSaved,
	}

	volumes, _ := r.buildVolumes(capsule)

	count := 0
	var found *corev1.Volume
	for i, v := range volumes {
		if v.Name == VolumeTypeSaved {
			count++
			found = &volumes[i]
		}
	}

	if count != 1 {
		t.Errorf("expected exactly 1 volume named %q, got %d", VolumeTypeSaved, count)
	}
	if found == nil || found.HostPath == nil || found.HostPath.Path != "/mnt/custom-saved" {
		t.Errorf("expected override volume to replace operator-built volume, got %+v", found)
	}
}

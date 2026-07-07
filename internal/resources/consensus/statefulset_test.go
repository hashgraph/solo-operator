// SPDX-License-Identifier: Apache-2.0

package consensus

import (
	"context"
	"fmt"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/events"
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
	return NewStatefulSetSubReconciler(c, scheme, nil)
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

	if sts.Spec.ServiceName != capsule.Name {
		t.Errorf("expected ServiceName %q, got %q", capsule.Name, sts.Spec.ServiceName)
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
						{Name: "settings-config"},
						{Name: "data"},
					},
				},
			},
		},
	}
}

func TestNeedsUpdate_NoChange(t *testing.T) {
	r := NewStatefulSetSubReconciler(fake.NewClientBuilder().Build(), runtime.NewScheme(), nil)
	sts := makeSts("v0.50.0", nil)
	if r.needsUpdate(sts, sts) {
		t.Error("identical StatefulSets should not need update")
	}
}

func TestNeedsUpdate_ImageChange(t *testing.T) {
	r := NewStatefulSetSubReconciler(fake.NewClientBuilder().Build(), runtime.NewScheme(), nil)
	existing := makeSts("v0.50.0", nil)
	desired := makeSts("v0.51.0", nil)
	if !r.needsUpdate(existing, desired) {
		t.Error("image change should trigger update")
	}
}

func TestNeedsUpdate_EnvVarAdded(t *testing.T) {
	r := NewStatefulSetSubReconciler(fake.NewClientBuilder().Build(), runtime.NewScheme(), nil)
	existing := makeSts("v0.50.0", nil)
	desired := makeSts("v0.50.0", []corev1.EnvVar{{Name: "JAVA_HEAP_MIN", Value: "4g"}})
	if !r.needsUpdate(existing, desired) {
		t.Error("adding an env var should trigger update")
	}
}

func TestNeedsUpdate_EnvVarValueChanged(t *testing.T) {
	r := NewStatefulSetSubReconciler(fake.NewClientBuilder().Build(), runtime.NewScheme(), nil)
	existing := makeSts("v0.50.0", []corev1.EnvVar{{Name: "JAVA_HEAP_MIN", Value: "4g"}})
	desired := makeSts("v0.50.0", []corev1.EnvVar{{Name: "JAVA_HEAP_MIN", Value: "8g"}})
	if !r.needsUpdate(existing, desired) {
		t.Error("env var value change should trigger update")
	}
}

func TestNeedsUpdate_InitContainerImageChange(t *testing.T) {
	r := NewStatefulSetSubReconciler(fake.NewClientBuilder().Build(), runtime.NewScheme(), nil)
	existing := makeSts("v0.50.0", nil)
	desired := makeSts("v0.50.0", nil)
	desired.Spec.Template.Spec.InitContainers[0].Image = "busybox:1.37.0"
	if !r.needsUpdate(existing, desired) {
		t.Error("init container image change should trigger update")
	}
}

func TestNeedsUpdate_VolumeAdded(t *testing.T) {
	r := NewStatefulSetSubReconciler(fake.NewClientBuilder().Build(), runtime.NewScheme(), nil)
	existing := makeSts("v0.50.0", nil)
	desired := makeSts("v0.50.0", nil)
	desired.Spec.Template.Spec.Volumes = append(desired.Spec.Template.Spec.Volumes,
		corev1.Volume{Name: "new-secret-vol"})
	if !r.needsUpdate(existing, desired) {
		t.Error("adding a volume should trigger update")
	}
}

func TestNeedsUpdate_VolumeSourceChanged(t *testing.T) {
	r := NewStatefulSetSubReconciler(fake.NewClientBuilder().Build(), runtime.NewScheme(), nil)
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
	r := NewStatefulSetSubReconciler(fake.NewClientBuilder().Build(), runtime.NewScheme(), nil)
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
	r := NewStatefulSetSubReconciler(fake.NewClientBuilder().Build(), runtime.NewScheme(), nil)
	existing := makeSts("v0.50.0", nil)
	desired := makeSts("v0.50.0", nil)
	desired.Spec.Template.Spec.InitContainers[0].Command = []string{"sh", "-c", "echo updated"}
	if !r.needsUpdate(existing, desired) {
		t.Error("init container command change should trigger update")
	}
}

func TestNeedsUpdate_ContainerSecurityContextChanged(t *testing.T) {
	r := NewStatefulSetSubReconciler(fake.NewClientBuilder().Build(), runtime.NewScheme(), nil)
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

// ── keys volume ──────────────────────────────────────────────────────────────

func capsuleWithSecretResources(signingSecretName, signingKeyName string) *operatorv1alpha1.ConsensusCapsule {
	c := capsuleWithImage("v0.50.0")
	c.Spec.Secrets = &operatorv1alpha1.ConsensusSecrets{
		SecretResources: &operatorv1alpha1.ConsensusSecretResources{
			SigningPrivateKey: operatorv1alpha1.KeyReference{
				SecretName: signingSecretName,
				KeyName:    signingKeyName,
			},
		},
	}
	return c
}

func findProjectedVolume(volumes []corev1.Volume, name string) *corev1.ProjectedVolumeSource {
	for _, v := range volumes {
		if v.Name == name && v.Projected != nil {
			return v.Projected
		}
	}
	return nil
}

func TestBuildVolumes_PublicKeysProjectedIntoKeysVolume(t *testing.T) {
	r := newStsReconciler()
	capsule := capsuleWithSecretResources("node0-keys", "s-private-node0.pem")

	volumes, mounts := r.buildVolumes(capsule)

	proj := findProjectedVolume(volumes, "keys")
	if proj == nil {
		t.Fatal("expected projected 'keys' volume")
	}

	var hasPublicKeys bool
	for _, src := range proj.Sources {
		if src.ConfigMap != nil && src.ConfigMap.Name == PublicKeysConfigMapName {
			hasPublicKeys = true
			if src.ConfigMap.Optional == nil || !*src.ConfigMap.Optional {
				t.Error("public-keys ConfigMap source should be optional")
			}
		}
	}
	if !hasPublicKeys {
		t.Errorf("expected public-keys ConfigMap in projected sources, got %+v", proj.Sources)
	}

	var hasKeysMount bool
	for _, m := range mounts {
		if m.Name == "keys" && m.MountPath == HapiBase+"/data/keys" {
			hasKeysMount = true
		}
	}
	if !hasKeysMount {
		t.Error("expected keys volume mount at data/keys")
	}
}

func TestBuildVolumes_PublicKeysProjectedAlongsidePrivateKey(t *testing.T) {
	r := newStsReconciler()
	capsule := capsuleWithSecretResources("node0-keys", "s-private-node0.pem")

	volumes, _ := r.buildVolumes(capsule)

	proj := findProjectedVolume(volumes, "keys")
	if proj == nil {
		t.Fatal("expected projected 'keys' volume")
	}

	var hasPrivateKey, hasPublicKeys bool
	for _, src := range proj.Sources {
		if src.Secret != nil && src.Secret.Name == "node0-keys" {
			hasPrivateKey = true
		}
		if src.ConfigMap != nil && src.ConfigMap.Name == PublicKeysConfigMapName {
			hasPublicKeys = true
		}
	}
	if !hasPrivateKey {
		t.Error("expected private key secret in projected sources")
	}
	if !hasPublicKeys {
		t.Error("expected public-keys ConfigMap in projected sources")
	}
}

func TestBuildVolumes_PublicKeysPresentEvenWithoutPrivateKeySecret(t *testing.T) {
	r := newStsReconciler()
	// SecretResources set but SigningPrivateKey left empty
	capsule := capsuleWithSecretResources("", "")

	volumes, _ := r.buildVolumes(capsule)

	proj := findProjectedVolume(volumes, "keys")
	if proj == nil {
		t.Fatal("expected projected 'keys' volume even without private key secret")
	}

	var hasPublicKeys bool
	for _, src := range proj.Sources {
		if src.ConfigMap != nil && src.ConfigMap.Name == PublicKeysConfigMapName {
			hasPublicKeys = true
		}
	}
	if !hasPublicKeys {
		t.Error("expected public-keys ConfigMap in projected sources even without private key")
	}
}

func TestBuildVolumes_PrivateKeyFilenameUsesCapsuleName(t *testing.T) {
	r := newStsReconciler()
	capsule := capsuleWithSecretResources("node0-keys", "s-private-node0.pem")
	// capsule.Name is "node0" from capsuleWithImage

	volumes, _ := r.buildVolumes(capsule)

	proj := findProjectedVolume(volumes, "keys")
	if proj == nil {
		t.Fatal("expected projected 'keys' volume")
	}

	for _, src := range proj.Sources {
		if src.Secret != nil {
			if len(src.Secret.Items) != 1 {
				t.Fatalf("expected 1 key mapping, got %d", len(src.Secret.Items))
			}
			want := fmt.Sprintf("s-private-%s.pem", capsule.Name)
			if got := src.Secret.Items[0].Path; got != want {
				t.Errorf("private key path: want %q, got %q", want, got)
			}
			return
		}
	}
	t.Error("no Secret projected source found")
}

// ── pod-level volume deduplication ───────────────────────────────────────────

func TestBuildVolumes_PodVolumeOverrideReplacesExisting(t *testing.T) {
	r := NewStatefulSetSubReconciler(fake.NewClientBuilder().Build(), runtime.NewScheme(), nil)
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

// ── sidecar skip / disable events ────────────────────────────────────────────

func newStsReconcilerWithRecorder(rec *events.FakeRecorder) *StatefulSetSubReconciler {
	scheme := newStsTestScheme()
	c := fake.NewClientBuilder().WithScheme(scheme).Build()
	return NewStatefulSetSubReconciler(c, scheme, rec)
}

func collectEvents(rec *events.FakeRecorder) []string {
	close(rec.Events)
	var out []string
	for e := range rec.Events {
		out = append(out, e)
	}
	return out
}

func countEventsContaining(evts []string, substr string) int {
	n := 0
	for _, e := range evts {
		if strings.Contains(e, substr) {
			n++
		}
	}
	return n
}

func TestAddSidecarContainers_NilSidecars_EmitsComponentSkipped(t *testing.T) {
	rec := events.NewFakeRecorder(32)
	r := newStsReconcilerWithRecorder(rec)

	capsule := capsuleWithImage("gcr.io/repo/cn:0.62.0")
	// All sidecar fields left nil (default from capsuleWithImage).
	sts := &appsv1.StatefulSet{}

	if err := r.addSidecarContainers(sts, capsule, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	evts := collectEvents(rec)
	for _, component := range []string{"blockStreamUploader", "recordStreamUploader", "eventStreamUploader", "backupUploader", "uc", "alloy"} {
		if got := countEventsContaining(evts, component); got != 1 {
			t.Errorf("expected one ComponentSkipped event for %s, got %d in %v", component, got, evts)
		}
	}
	if got := countEventsContaining(evts, operatorv1alpha1.EventSidecarDisabled); got != 0 {
		t.Errorf("expected no SidecarDisabled events, got %d in %v", got, evts)
	}
}

func TestAddSidecarContainers_DisabledSidecars_EmitsSidecarDisabled(t *testing.T) {
	rec := events.NewFakeRecorder(32)
	r := newStsReconcilerWithRecorder(rec)

	disabled := false
	capsule := capsuleWithImage("gcr.io/repo/cn:0.62.0")
	capsule.Spec.PodProperties.Containers.BlocksUploader = &operatorv1alpha1.CheetahUploader{Enabled: disabled}
	capsule.Spec.PodProperties.Containers.RecordsUploader = &operatorv1alpha1.CheetahUploader{Enabled: disabled}
	capsule.Spec.PodProperties.Containers.EventsUploader = &operatorv1alpha1.CheetahUploader{Enabled: disabled}
	capsule.Spec.PodProperties.Containers.BackupUploader = &operatorv1alpha1.StreamUploader{Enabled: disabled}
	capsule.Spec.PodProperties.Containers.UC = &operatorv1alpha1.UcSidecar{Enabled: disabled}
	sts := &appsv1.StatefulSet{}

	if err := r.addSidecarContainers(sts, capsule, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	evts := collectEvents(rec)
	for _, component := range []string{"blockStreamUploader", "recordStreamUploader", "eventStreamUploader", "backupUploader", "uc"} {
		if got := countEventsContaining(evts, component); got != 1 {
			t.Errorf("expected one SidecarDisabled event for %s, got %d in %v", component, got, evts)
		}
	}
	if got := countEventsContaining(evts, operatorv1alpha1.EventSidecarDisabled); got != 5 {
		t.Errorf("expected 5 SidecarDisabled events, got %d in %v", got, evts)
	}
	if got := countEventsContaining(evts, "alloy"); got != 1 {
		t.Errorf("expected one ComponentSkipped event for alloy, got %d in %v", got, evts)
	}
}

func TestAddSidecarContainers_EnabledSidecars_NoSkipOrDisableEvents(t *testing.T) {
	rec := events.NewFakeRecorder(32)
	r := newStsReconcilerWithRecorder(rec)

	capsule := capsuleWithImage("gcr.io/repo/cn:0.62.0")
	capsule.Spec.AccountId = "0.0.3"
	enabled := true
	capsule.Spec.PodProperties.Containers.BlocksUploader = &operatorv1alpha1.CheetahUploader{
		Enabled:         enabled,
		SoftwareVersion: &operatorv1alpha1.SoftwareVersion{Repository: "gcr.io", ImageName: "cheetah", ImageTag: "1.0"},
	}
	capsule.Spec.PodProperties.Containers.RecordsUploader = &operatorv1alpha1.CheetahUploader{
		Enabled:         enabled,
		SoftwareVersion: &operatorv1alpha1.SoftwareVersion{Repository: "gcr.io", ImageName: "cheetah", ImageTag: "1.0"},
	}
	capsule.Spec.PodProperties.Containers.EventsUploader = &operatorv1alpha1.CheetahUploader{
		Enabled:         enabled,
		SoftwareVersion: &operatorv1alpha1.SoftwareVersion{Repository: "gcr.io", ImageName: "cheetah", ImageTag: "1.0"},
	}
	capsule.Spec.PodProperties.Containers.BackupUploader = &operatorv1alpha1.StreamUploader{
		Enabled:         enabled,
		SoftwareVersion: &operatorv1alpha1.SoftwareVersion{Repository: "gcr.io", ImageName: "backup", ImageTag: "1.0"},
	}
	capsule.Spec.PodProperties.Containers.UC = &operatorv1alpha1.UcSidecar{
		Enabled:         enabled,
		SoftwareVersion: &operatorv1alpha1.SoftwareVersion{Repository: "gcr.io", ImageName: "uc", ImageTag: "1.0"},
	}
	alloyEnabled := &operatorv1alpha1.AlloyConfig{Enabled: true,
		SoftwareVersion: &operatorv1alpha1.SoftwareVersion{Repository: "gcr.io", ImageName: "alloy", ImageTag: "1.0"},
	}

	sts := &appsv1.StatefulSet{Spec: appsv1.StatefulSetSpec{Template: corev1.PodTemplateSpec{}}}
	if err := r.addSidecarContainers(sts, capsule, alloyEnabled); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	evts := collectEvents(rec)
	if got := countEventsContaining(evts, operatorv1alpha1.EventComponentSkipped); got != 0 {
		t.Errorf("expected no ComponentSkipped events, got %d in %v", got, evts)
	}
	if got := countEventsContaining(evts, operatorv1alpha1.EventSidecarDisabled); got != 0 {
		t.Errorf("expected no SidecarDisabled events, got %d in %v", got, evts)
	}
}

func TestBuildUcContainer_Defaults(t *testing.T) {
	r := newStsReconciler()
	config := &operatorv1alpha1.UcSidecar{
		SoftwareVersion: &operatorv1alpha1.SoftwareVersion{Repository: "gcr.io", ImageName: "uc", ImageTag: "1.0"},
		Enabled:         true,
	}
	c := r.buildUcContainer(config)
	if c.Name != "uc" {
		t.Errorf("name: got %q, want %q", c.Name, "uc")
	}
	if c.Image != "gcr.io/uc:1.0" {
		t.Errorf("image: got %q, want %q", c.Image, "gcr.io/uc:1.0")
	}
	if c.ImagePullPolicy != corev1.PullIfNotPresent {
		t.Errorf("imagePullPolicy: got %q, want IfNotPresent", c.ImagePullPolicy)
	}
}

func TestBuildUcContainer_CustomNameAndImage(t *testing.T) {
	r := newStsReconciler()
	config := &operatorv1alpha1.UcSidecar{
		Name:            "my-uc",
		SoftwareVersion: &operatorv1alpha1.SoftwareVersion{Repository: "custom.io", ImageName: "uc", ImageTag: "latest"},
		Enabled:         true,
	}
	c := r.buildUcContainer(config)
	if c.Name != "my-uc" {
		t.Errorf("name: got %q, want %q", c.Name, "my-uc")
	}
	if c.Image != "custom.io/uc:latest" {
		t.Errorf("image: got %q, want %q", c.Image, "custom.io/uc:latest")
	}
}

func TestBuildUcContainer_AdditionalVolumeMounts(t *testing.T) {
	r := newStsReconciler()
	config := &operatorv1alpha1.UcSidecar{
		SoftwareVersion: &operatorv1alpha1.SoftwareVersion{Repository: "gcr.io", ImageName: "uc", ImageTag: "1.0"},
		Enabled:         true,
		AdditionalVolumeMounts: []corev1.VolumeMount{
			{Name: "data", MountPath: "/data"},
		},
	}
	c := r.buildUcContainer(config)
	if len(c.VolumeMounts) != 1 || c.VolumeMounts[0].Name != "data" {
		t.Errorf("volumeMounts: got %v", c.VolumeMounts)
	}
}

func TestBuildUcContainer_NoSoftwareVersion_UsesDefault(t *testing.T) {
	r := newStsReconciler()
	config := &operatorv1alpha1.UcSidecar{Enabled: true}
	c := r.buildUcContainer(config)
	if c.Image != UCDefaultImage() {
		t.Errorf("image: got %q, want default %q", c.Image, UCDefaultImage())
	}
}

func containerNames(sts *appsv1.StatefulSet) map[string]bool {
	out := map[string]bool{}
	for _, c := range sts.Spec.Template.Spec.Containers {
		out[c.Name] = true
	}
	return out
}

func TestBuildStatefulSet_OmittedSidecarRemoved(t *testing.T) {
	r := newStsReconciler()

	capsule := capsuleWithImage("gcr.io/repo/cn:0.62.0")
	capsule.Spec.AccountId = "0.0.3"
	capsule.Spec.PodProperties.Containers.UC = &operatorv1alpha1.UcSidecar{
		Enabled:         true,
		SoftwareVersion: &operatorv1alpha1.SoftwareVersion{Repository: "gcr.io", ImageName: "uc", ImageTag: "1.0"},
	}

	withUC, err := r.buildStatefulSet(context.Background(), capsule)
	if err != nil {
		t.Fatalf("buildStatefulSet (with uc): %v", err)
	}
	if !containerNames(withUC)["uc"] {
		t.Fatalf("expected uc container present, got %v", containerNames(withUC))
	}

	capsule.Spec.PodProperties.Containers.UC = nil
	withoutUC, err := r.buildStatefulSet(context.Background(), capsule)
	if err != nil {
		t.Fatalf("buildStatefulSet (without uc): %v", err)
	}
	if containerNames(withoutUC)["uc"] {
		t.Errorf("expected uc container removed when omitted, got %v", containerNames(withoutUC))
	}
}

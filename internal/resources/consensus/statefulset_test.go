// SPDX-License-Identifier: Apache-2.0

package consensus

import (
	"context"
	"maps"
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
	"github.com/hashgraph/solo-operator/internal/uc"
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

func TestBuildStatefulSet_AutoStartEnvVar(t *testing.T) {
	cases := map[string]struct {
		startPolicy operatorv1alpha1.StartPolicy
		want        string
	}{
		"unset_defaults_true": {startPolicy: "", want: "true"},
		"auto":                {startPolicy: operatorv1alpha1.StartPolicyAuto, want: "true"},
		"manual":              {startPolicy: operatorv1alpha1.StartPolicyManual, want: "false"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := newStsReconciler()
			capsule := capsuleWithImage("v0.50.0")
			capsule.Spec.StartPolicy = tc.startPolicy

			sts, err := r.buildStatefulSet(context.Background(), capsule)
			if err != nil {
				t.Fatalf("buildStatefulSet: %v", err)
			}

			var got string
			found := false
			for _, e := range sts.Spec.Template.Spec.Containers[0].Env {
				if e.Name == "AUTO_START_CONSENSUS_SERVICE" {
					got, found = e.Value, true
				}
			}
			if !found {
				t.Fatal("AUTO_START_CONSENSUS_SERVICE env var not set")
			}
			if got != tc.want {
				t.Errorf("AUTO_START_CONSENSUS_SERVICE: want %q, got %q", tc.want, got)
			}
		})
	}
}

func TestBuildStatefulSet_BlockNodesMount(t *testing.T) {
	r := newStsReconciler()
	capsule := capsuleWithImage("v0.50.0")
	capsule.Spec.BlockNodesConfigRef = "node0-block-nodes" // ref triggers the configmap volume

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
		t.Error("expected block-nodes-config volume when BlockNodesConfigRef is set")
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

// ── imagePullSecrets ───────────────────────────────────────────────────────────

func hasPullSecret(secrets []corev1.LocalObjectReference, name string) bool {
	for _, s := range secrets {
		if s.Name == name {
			return true
		}
	}
	return false
}

// TestBuildStatefulSet_ImagePullSecretsFromResolvedAlloy is a regression test for the
// case where Alloy is configured only via Orbit defaults (the capsule declares no alloy
// block). The Alloy container is built from the *resolved* config, so its pull secret must
// be collected from the resolved Alloy SoftwareVersion rather than the raw capsule spec —
// otherwise the Alloy sidecar pulls a private image with no credential (ImagePullBackOff).
func TestBuildStatefulSet_ImagePullSecretsFromResolvedAlloy(t *testing.T) {
	scheme := newStsTestScheme()
	orbit := &operatorv1alpha1.Orbit{
		ObjectMeta: metav1.ObjectMeta{Name: "test-orbit"},
		Spec: operatorv1alpha1.OrbitSpec{
			Telemetry: &operatorv1alpha1.OrbitTelemetry{
				Alloy: &operatorv1alpha1.OrbitAlloyDefaults{
					Enabled: true,
					SoftwareVersion: &operatorv1alpha1.SoftwareVersion{
						Repository:       "private.registry.io",
						ImageName:        "alloy",
						ImageTag:         "v1.14.2",
						ImagePullSecrets: []corev1.LocalObjectReference{{Name: "orbit-alloy-creds"}},
					},
				},
			},
		},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(orbit).Build()
	r := NewStatefulSetSubReconciler(c, scheme, nil)

	// Capsule references the orbit but defines NO alloy block of its own, so the Alloy
	// SoftwareVersion (and its pull secret) comes entirely from the orbit defaults.
	capsule := capsuleWithImage("v0.50.0")

	sts, err := r.buildStatefulSet(context.Background(), capsule)
	if err != nil {
		t.Fatalf("buildStatefulSet: %v", err)
	}

	if !hasPullSecret(sts.Spec.Template.Spec.ImagePullSecrets, "orbit-alloy-creds") {
		t.Errorf("expected pod imagePullSecrets to include orbit-resolved Alloy secret %q, got %v",
			"orbit-alloy-creds", sts.Spec.Template.Spec.ImagePullSecrets)
	}
}

// TestBuildStatefulSet_ImagePullSecretsMergedAndDeduped verifies that pull secrets from
// multiple enabled containers are merged onto the pod template and deduplicated by name.
func TestBuildStatefulSet_ImagePullSecretsMergedAndDeduped(t *testing.T) {
	r := newStsReconciler()
	capsule := capsuleWithImage("v0.50.0")
	capsule.Spec.PodProperties.Containers.ConsensusNode.SoftwareVersion.ImagePullSecrets =
		[]corev1.LocalObjectReference{{Name: "shared-creds"}, {Name: "cn-creds"}}
	capsule.Spec.PodProperties.Containers.BackupUploader = &operatorv1alpha1.StreamUploader{
		Enabled: true,
		SoftwareVersion: &operatorv1alpha1.SoftwareVersion{
			Repository:       "private.registry.io",
			ImageName:        "backup",
			ImageTag:         "v1",
			ImagePullSecrets: []corev1.LocalObjectReference{{Name: "shared-creds"}, {Name: "backup-creds"}},
		},
	}

	sts, err := r.buildStatefulSet(context.Background(), capsule)
	if err != nil {
		t.Fatalf("buildStatefulSet: %v", err)
	}

	got := sts.Spec.Template.Spec.ImagePullSecrets
	for _, name := range []string{"shared-creds", "cn-creds", "backup-creds"} {
		if !hasPullSecret(got, name) {
			t.Errorf("expected merged imagePullSecrets to include %q, got %v", name, got)
		}
	}
	if len(got) != 3 {
		t.Errorf("expected 3 deduplicated imagePullSecrets, got %d: %v", len(got), got)
	}
}

// TestBuildStatefulSet_NoImagePullSecrets verifies the pod template leaves imagePullSecrets
// unset when no container declares any.
func TestBuildStatefulSet_NoImagePullSecrets(t *testing.T) {
	r := newStsReconciler()
	capsule := capsuleWithImage("v0.50.0")

	sts, err := r.buildStatefulSet(context.Background(), capsule)
	if err != nil {
		t.Fatalf("buildStatefulSet: %v", err)
	}

	if len(sts.Spec.Template.Spec.ImagePullSecrets) != 0 {
		t.Errorf("expected no imagePullSecrets, got %v", sts.Spec.Template.Spec.ImagePullSecrets)
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

	volumes, mounts := r.buildVolumes(capsule, false)

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

	volumes, _ := r.buildVolumes(capsule, false)

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

	volumes, _ := r.buildVolumes(capsule, false)

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

func TestBuildVolumes_PrivateKeyPathDerivedFromNodeId(t *testing.T) {
	r := newStsReconciler()
	// Secret key is named "private.pem" — operator must mount it as s-private-node1.pem
	// because nodeId=0 → formatNodeName offset+1 = "node1".
	capsule := capsuleWithSecretResources("node0-keys", "private.pem")
	capsule.Spec.NodeId = 0

	volumes, _ := r.buildVolumes(capsule, false)

	proj := findProjectedVolume(volumes, "keys")
	if proj == nil {
		t.Fatal("expected projected 'keys' volume")
	}

	for _, src := range proj.Sources {
		if src.Secret == nil {
			continue
		}
		if len(src.Secret.Items) != 1 {
			t.Fatalf("expected 1 key mapping, got %d", len(src.Secret.Items))
		}
		item := src.Secret.Items[0]
		if item.Key != "private.pem" {
			t.Errorf("secret Key: want %q, got %q", "private.pem", item.Key)
		}
		// Path must be s-private-node<nodeId+1>.pem, not the secret key name.
		if item.Path != "s-private-node1.pem" {
			t.Errorf("secret Path: want %q, got %q", "s-private-node1.pem", item.Path)
		}
		return
	}
	t.Error("no Secret projected source found")
}

func TestBuildVolumes_PrivateKeyPathZeroBasedGate(t *testing.T) {
	r := newStsReconciler()
	// nodeNameZeroBased=true → nodeId=0 mounts as s-private-node0.pem (no +1).
	capsule := capsuleWithSecretResources("node0-keys", "private.pem")
	capsule.Spec.NodeId = 0

	volumes, _ := r.buildVolumes(capsule, true)

	proj := findProjectedVolume(volumes, "keys")
	if proj == nil {
		t.Fatal("expected projected 'keys' volume")
	}

	for _, src := range proj.Sources {
		if src.Secret == nil {
			continue
		}
		item := src.Secret.Items[0]
		if item.Path != "s-private-node0.pem" {
			t.Errorf("secret Path: want %q, got %q", "s-private-node0.pem", item.Path)
		}
		return
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

	volumes, _ := r.buildVolumes(capsule, false)

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

	if err := r.addSidecarContainers(context.Background(), sts, capsule, nil, uc.ModeClusterOnly); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	evts := collectEvents(rec)
	for _, component := range []string{"blockStreamUploader", "recordStreamUploader", "eventStreamUploader", "backupUploader", "uc", "alloy"} {
		if got := countEventsContaining(evts, component); got != 1 {
			t.Errorf("expected one ComponentSkipped event for %s, got %d in %v", component, got, evts)
		}
	}
	if got := countEventsContaining(evts, operatorv1alpha1.EventSidecarDisabled.String()); got != 0 {
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

	if err := r.addSidecarContainers(context.Background(), sts, capsule, nil, uc.ModeClusterOnly); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	evts := collectEvents(rec)
	for _, component := range []string{"blockStreamUploader", "recordStreamUploader", "eventStreamUploader", "backupUploader", "uc"} {
		if got := countEventsContaining(evts, component); got != 1 {
			t.Errorf("expected one SidecarDisabled event for %s, got %d in %v", component, got, evts)
		}
	}
	if got := countEventsContaining(evts, operatorv1alpha1.EventSidecarDisabled.String()); got != 5 {
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
	if err := r.addSidecarContainers(context.Background(), sts, capsule, alloyEnabled, uc.ModeClusterOnly); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	evts := collectEvents(rec)
	if got := countEventsContaining(evts, operatorv1alpha1.EventComponentSkipped.String()); got != 0 {
		t.Errorf("expected no ComponentSkipped events, got %d in %v", got, evts)
	}
	if got := countEventsContaining(evts, operatorv1alpha1.EventSidecarDisabled.String()); got != 0 {
		t.Errorf("expected no SidecarDisabled events, got %d in %v", got, evts)
	}
}

func TestBuildUcContainer_Defaults(t *testing.T) {
	r := newStsReconciler()
	capsule := capsuleWithImage("v0.50.0")
	config := &operatorv1alpha1.UcSidecar{
		SoftwareVersion: &operatorv1alpha1.SoftwareVersion{Repository: "gcr.io", ImageName: "uc", ImageTag: "1.0"},
		Enabled:         true,
	}
	c := r.buildUcContainer(capsule, config, uc.ModeClusterOnly)
	if c.Name != "uc" {
		t.Errorf("name: got %q, want %q", c.Name, "uc")
	}
	if c.Image != "gcr.io/uc:1.0" {
		t.Errorf("image: got %q, want %q", c.Image, "gcr.io/uc:1.0")
	}
	if c.ImagePullPolicy != corev1.PullIfNotPresent {
		t.Errorf("imagePullPolicy: got %q, want IfNotPresent", c.ImagePullPolicy)
	}
	// The UC must run as hedera (2000:2000) to share the upgrade PVC with the
	// consensus node — see buildUcContainer.
	assertRunAs(t, c, HederaUID)
	assertRestricted(t, c)
}

func TestBuildUcContainer_EnvVars(t *testing.T) {
	r := newStsReconciler()
	capsule := capsuleWithImage("v0.50.0")
	capsule.Spec.NodeId = 3
	capsule.Spec.Orbit = "my-orbit"
	config := &operatorv1alpha1.UcSidecar{Enabled: true}
	c := r.buildUcContainer(capsule, config, uc.ModeClusterOnly)

	envMap := map[string]string{}
	for _, e := range c.Env {
		if e.Value != "" {
			envMap[e.Name] = e.Value
		}
	}
	if envMap["UC_NODE_ID"] != "3" {
		t.Errorf("UC_NODE_ID: got %q, want %q", envMap["UC_NODE_ID"], "3")
	}
	if envMap["UC_ORBIT"] != "my-orbit" {
		t.Errorf("UC_ORBIT: got %q, want %q", envMap["UC_ORBIT"], "my-orbit")
	}
	if envMap["UC_MODE"] != string(uc.ModeClusterOnly) {
		t.Errorf("UC_MODE: got %q, want %q", envMap["UC_MODE"], uc.ModeClusterOnly)
	}
	foundNamespace := false
	for _, e := range c.Env {
		if e.Name == "UC_NAMESPACE" && e.ValueFrom != nil && e.ValueFrom.FieldRef != nil {
			if e.ValueFrom.FieldRef.FieldPath == "metadata.namespace" {
				foundNamespace = true
			}
		}
	}
	if !foundNamespace {
		t.Error("UC_NAMESPACE downward-API env var not found")
	}
}

func TestBuildUcContainer_ModeMainnet(t *testing.T) {
	r := newStsReconciler()
	capsule := capsuleWithImage("v0.50.0")
	config := &operatorv1alpha1.UcSidecar{Enabled: true}
	c := r.buildUcContainer(capsule, config, uc.ModeMainnet)
	for _, e := range c.Env {
		if e.Name == "UC_MODE" && e.Value == string(uc.ModeMainnet) {
			return
		}
	}
	t.Errorf("UC_MODE=mainnet not found in env")
}

func TestBuildUcContainer_VolumeMounts(t *testing.T) {
	r := newStsReconciler()
	capsule := capsuleWithImage("v0.50.0")
	config := &operatorv1alpha1.UcSidecar{Enabled: true}
	c := r.buildUcContainer(capsule, config, uc.ModeClusterOnly)

	mountPaths := map[string]string{}
	for _, m := range c.VolumeMounts {
		mountPaths[m.Name] = m.MountPath
	}
	if mountPaths[VolumeTypeUpgrade] != HapiBase+"/data/upgrade" {
		t.Errorf("upgrade mount: got %q", mountPaths[VolumeTypeUpgrade])
	}
	if mountPaths[VolumeTypeUCData] != UCDataPath {
		t.Errorf("uc-data mount: got %q, want %q", mountPaths[VolumeTypeUCData], UCDataPath)
	}
}

func TestBuildUcContainer_CustomNameAndImage(t *testing.T) {
	r := newStsReconciler()
	capsule := capsuleWithImage("v0.50.0")
	config := &operatorv1alpha1.UcSidecar{
		Name:            "my-uc",
		SoftwareVersion: &operatorv1alpha1.SoftwareVersion{Repository: "custom.io", ImageName: "uc", ImageTag: "latest"},
		Enabled:         true,
	}
	c := r.buildUcContainer(capsule, config, uc.ModeClusterOnly)
	if c.Name != "my-uc" {
		t.Errorf("name: got %q, want %q", c.Name, "my-uc")
	}
	if c.Image != "custom.io/uc:latest" {
		t.Errorf("image: got %q, want %q", c.Image, "custom.io/uc:latest")
	}
}

func TestBuildUcContainer_AdditionalVolumeMounts(t *testing.T) {
	r := newStsReconciler()
	capsule := capsuleWithImage("v0.50.0")
	config := &operatorv1alpha1.UcSidecar{
		SoftwareVersion: &operatorv1alpha1.SoftwareVersion{Repository: "gcr.io", ImageName: "uc", ImageTag: "1.0"},
		Enabled:         true,
		AdditionalVolumeMounts: []corev1.VolumeMount{
			{Name: "data", MountPath: "/data"},
		},
	}
	c := r.buildUcContainer(capsule, config, uc.ModeClusterOnly)
	// should have upgrade + uc-data + logs (RO) + the additional mount
	if len(c.VolumeMounts) != 4 {
		t.Errorf("volumeMounts: expected 4 (upgrade + uc-data + logs + data), got %d: %v", len(c.VolumeMounts), c.VolumeMounts)
	}
	last := c.VolumeMounts[len(c.VolumeMounts)-1]
	if last.Name != "data" {
		t.Errorf("last volumeMount: got %q, want %q", last.Name, "data")
	}
}

func TestBuildUcContainer_PollIntervals_Defaults(t *testing.T) {
	r := newStsReconciler()
	capsule := capsuleWithImage("v0.50.0")
	config := &operatorv1alpha1.UcSidecar{Enabled: true}
	c := r.buildUcContainer(capsule, config, uc.ModeClusterOnly)
	for _, e := range c.Env {
		if e.Name == "UC_POLL_INTERVAL" || e.Name == "UC_FREEZE_POLL_INTERVAL" || e.Name == "UC_PLATFORM_POLL_INTERVAL" {
			t.Errorf("env %s should not be set when omitted from spec", e.Name)
		}
	}
}

func TestBuildUcContainer_PollIntervals_Custom(t *testing.T) {
	r := newStsReconciler()
	capsule := capsuleWithImage("v0.50.0")
	config := &operatorv1alpha1.UcSidecar{
		Enabled:              true,
		PollInterval:         "10s",
		FreezePollInterval:   "100ms",
		PlatformPollInterval: "2s",
	}
	c := r.buildUcContainer(capsule, config, uc.ModeClusterOnly)
	envMap := map[string]string{}
	for _, e := range c.Env {
		if e.Value != "" {
			envMap[e.Name] = e.Value
		}
	}
	if envMap["UC_POLL_INTERVAL"] != "10s" {
		t.Errorf("UC_POLL_INTERVAL: got %q, want %q", envMap["UC_POLL_INTERVAL"], "10s")
	}
	if envMap["UC_FREEZE_POLL_INTERVAL"] != "100ms" {
		t.Errorf("UC_FREEZE_POLL_INTERVAL: got %q, want %q", envMap["UC_FREEZE_POLL_INTERVAL"], "100ms")
	}
	if envMap["UC_PLATFORM_POLL_INTERVAL"] != "2s" {
		t.Errorf("UC_PLATFORM_POLL_INTERVAL: got %q, want %q", envMap["UC_PLATFORM_POLL_INTERVAL"], "2s")
	}
}

func TestBuildUcContainer_UsesExplicitImage(t *testing.T) {
	r := newStsReconciler()
	capsule := capsuleWithImage("v0.50.0")
	// The operator has no built-in UC default; the image must be declared explicitly.
	config := &operatorv1alpha1.UcSidecar{
		Enabled: true,
		SoftwareVersion: &operatorv1alpha1.SoftwareVersion{
			Repository: "ghcr.io/hashgraph",
			ImageName:  "solo-operator-uc",
			ImageTag:   "0.5.0",
		},
	}
	c := r.buildUcContainer(capsule, config, uc.ModeClusterOnly)
	if want := "ghcr.io/hashgraph/solo-operator-uc:0.5.0"; c.Image != want {
		t.Errorf("image: got %q, want %q", c.Image, want)
	}
}

func TestBuildStatefulSet_UCEnabled_EmptyDirVolume(t *testing.T) {
	r := newStsReconciler()
	capsule := capsuleWithImage("gcr.io/repo/cn:0.62.0")
	capsule.Spec.PodProperties.Containers.UC = &operatorv1alpha1.UcSidecar{
		Enabled:         true,
		SoftwareVersion: &operatorv1alpha1.SoftwareVersion{Repository: "gcr.io", ImageName: "uc", ImageTag: "1.0"},
		// Storage nil → emptyDir
	}
	sts, err := r.buildStatefulSet(context.Background(), capsule)
	if err != nil {
		t.Fatalf("buildStatefulSet: %v", err)
	}
	var ucVol *corev1.Volume
	for i := range sts.Spec.Template.Spec.Volumes {
		if sts.Spec.Template.Spec.Volumes[i].Name == VolumeTypeUCData {
			ucVol = &sts.Spec.Template.Spec.Volumes[i]
			break
		}
	}
	if ucVol == nil {
		t.Fatalf("expected %q volume, not found", VolumeTypeUCData)
	}
	if ucVol.EmptyDir == nil {
		t.Errorf("expected emptyDir when Storage is nil, got %+v", ucVol.VolumeSource)
	}
}

func TestBuildStatefulSet_UCEnabled_PVCVolume(t *testing.T) {
	r := newStsReconciler()
	capsule := capsuleWithImage("gcr.io/repo/cn:0.62.0")
	storageClass := "standard"
	capsule.Spec.PodProperties.Containers.UC = &operatorv1alpha1.UcSidecar{Enabled: true}
	capsule.Spec.PersistentVolumeClaims = &operatorv1alpha1.ConsensusPVCConfig{
		UCData: &corev1.PersistentVolumeClaimSpec{
			StorageClassName: &storageClass,
		},
	}
	sts, err := r.buildStatefulSet(context.Background(), capsule)
	if err != nil {
		t.Fatalf("buildStatefulSet: %v", err)
	}
	var ucVol *corev1.Volume
	for i := range sts.Spec.Template.Spec.Volumes {
		if sts.Spec.Template.Spec.Volumes[i].Name == VolumeTypeUCData {
			ucVol = &sts.Spec.Template.Spec.Volumes[i]
			break
		}
	}
	if ucVol == nil {
		t.Fatalf("expected %q volume, not found", VolumeTypeUCData)
	}
	if ucVol.PersistentVolumeClaim == nil {
		t.Errorf("expected PVC-backed volume when Storage is set, got %+v", ucVol.VolumeSource)
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

func TestBuildBackupUploaderContainer_EnvVars(t *testing.T) {
	tests := []struct {
		name string
		cfg  *operatorv1alpha1.StreamUploaderConfig
		want map[string]string
	}{
		{
			name: "s3 bare bucket",
			cfg: &operatorv1alpha1.StreamUploaderConfig{
				BucketName: "solo-backups",
				S3:         &operatorv1alpha1.S3Config{Enabled: true, URL: "http://rustfs:9000"},
			},
			want: map[string]string{
				"BACKUP_UPLOADER_BUCKET_1":       "solo-backups",
				"RCLONE_CONFIG_BACKUPS_TYPE":     "s3",
				"RCLONE_CONFIG_BACKUPS_ENDPOINT": "http://rustfs:9000",
			},
		},
		{
			name: "gcs bucket with folder",
			cfg: &operatorv1alpha1.StreamUploaderConfig{
				BucketName: "solo-backups/node0",
				GCS:        &operatorv1alpha1.GCSConfig{Enabled: true},
			},
			want: map[string]string{
				"BACKUP_UPLOADER_BUCKET_1":   "solo-backups/node0",
				"RCLONE_CONFIG_BACKUPS_TYPE": "gcs",
			},
		},
		{
			name: "no bucket",
			cfg: &operatorv1alpha1.StreamUploaderConfig{
				S3: &operatorv1alpha1.S3Config{Enabled: true},
			},
			want: map[string]string{
				"RCLONE_CONFIG_BACKUPS_TYPE": "s3",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newStsReconciler()
			c := r.buildBackupUploaderContainer(&operatorv1alpha1.StreamUploader{
				Enabled:         true,
				SoftwareVersion: &operatorv1alpha1.SoftwareVersion{Repository: "gcr.io", ImageName: "backup", ImageTag: "1.0"},
				Config:          tt.cfg,
			})

			got := map[string]string{}
			for _, e := range c.Env {
				got[e.Name] = e.Value
			}
			if !maps.Equal(got, tt.want) {
				t.Errorf("env: got %v, want %v", got, tt.want)
			}
		})
	}
}

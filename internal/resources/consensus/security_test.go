// SPDX-License-Identifier: Apache-2.0

package consensus

import (
	"context"
	"slices"
	"testing"

	corev1 "k8s.io/api/core/v1"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

func assertRestricted(t *testing.T, c corev1.Container) {
	t.Helper()
	sc := c.SecurityContext
	if sc == nil {
		t.Fatalf("%s: securityContext is nil", c.Name)
	}
	if sc.AllowPrivilegeEscalation == nil || *sc.AllowPrivilegeEscalation {
		t.Errorf("%s: allowPrivilegeEscalation must be false, got %v", c.Name, sc.AllowPrivilegeEscalation)
	}
	if sc.Capabilities == nil || !slices.Equal(sc.Capabilities.Drop, []corev1.Capability{"ALL"}) {
		t.Errorf("%s: capabilities.drop must be [ALL], got %+v", c.Name, sc.Capabilities)
	}
}

func assertRunAs(t *testing.T, c corev1.Container, uid int64) {
	t.Helper()
	sc := c.SecurityContext
	if sc == nil || sc.RunAsUser == nil || sc.RunAsGroup == nil {
		t.Fatalf("%s: securityContext must pin runAsUser/runAsGroup, got %+v", c.Name, sc)
	}
	if *sc.RunAsUser != uid || *sc.RunAsGroup != uid {
		t.Errorf("%s: runAsUser/runAsGroup: got %d/%d, want %d/%d", c.Name, *sc.RunAsUser, *sc.RunAsGroup, uid, uid)
	}
}

func capsuleWithAllSidecars() *operatorv1alpha1.ConsensusCapsule {
	capsule := capsuleWithImage("v0.72.1")
	capsule.Spec.AccountId = "0.0.3"
	sv := func(name string) *operatorv1alpha1.SoftwareVersion {
		return &operatorv1alpha1.SoftwareVersion{Repository: "gcr.io", ImageName: name, ImageTag: "1.0"}
	}
	containers := &capsule.Spec.PodProperties.Containers
	containers.BlocksUploader = &operatorv1alpha1.CheetahUploader{Enabled: true, SoftwareVersion: sv("cheetah")}
	containers.RecordsUploader = &operatorv1alpha1.CheetahUploader{Enabled: true, SoftwareVersion: sv("cheetah")}
	containers.EventsUploader = &operatorv1alpha1.CheetahUploader{Enabled: true, SoftwareVersion: sv("cheetah")}
	containers.BackupUploader = &operatorv1alpha1.StreamUploader{Enabled: true, SoftwareVersion: sv("backup")}
	containers.UC = &operatorv1alpha1.UcSidecar{Enabled: true, SoftwareVersion: sv("uc")}
	containers.Alloy = &operatorv1alpha1.AlloyConfig{Enabled: true, SoftwareVersion: sv("alloy")}
	return capsule
}

func TestBuildStatefulSet_HardenedByDefault(t *testing.T) {
	r := newStsReconciler()
	sts, err := r.buildStatefulSet(context.Background(), capsuleWithAllSidecars())
	if err != nil {
		t.Fatalf("buildStatefulSet: %v", err)
	}
	spec := sts.Spec.Template.Spec

	psc := spec.SecurityContext
	if psc == nil {
		t.Fatal("pod securityContext is nil")
	}
	if psc.SeccompProfile == nil || psc.SeccompProfile.Type != corev1.SeccompProfileTypeRuntimeDefault {
		t.Errorf("pod seccompProfile must be RuntimeDefault, got %+v", psc.SeccompProfile)
	}
	if psc.RunAsNonRoot == nil || !*psc.RunAsNonRoot {
		t.Errorf("pod runAsNonRoot must be true, got %v", psc.RunAsNonRoot)
	}
	if psc.FSGroup == nil || *psc.FSGroup != HederaUID {
		t.Errorf("pod fsGroup must stay %d with cheetah enabled, got %v", HederaUID, psc.FSGroup)
	}

	all := append(slices.Clone(spec.InitContainers), spec.Containers...)
	if len(all) != 8 {
		t.Fatalf("expected 8 containers (genesis-init, node, 3 cheetah, backup, uc, alloy), got %d", len(all))
	}
	for _, c := range all {
		assertRestricted(t, c)
	}

	byName := map[string]corev1.Container{}
	for _, c := range all {
		byName[c.Name] = c
	}
	for _, name := range []string{"genesis-init", "blockstream-uploader", "record-stream-uploader", "event-stream-uploader", "uc"} {
		assertRunAs(t, byName[name], HederaUID)
	}
	assertRunAs(t, byName["alloy"], AlloyUID)

	// Root exception for the /<hostname>.txt marker; flip to HederaUID once hiero-ledger/solo-containers#255 ships.
	backup := byName["backup-uploader"].SecurityContext
	if backup.RunAsNonRoot == nil || *backup.RunAsNonRoot {
		t.Errorf("backup-uploader must opt out of runAsNonRoot, got %v", backup.RunAsNonRoot)
	}
	if main := byName[DefaultConsensusContainerName].SecurityContext; main.RunAsUser != nil || main.RunAsNonRoot != nil {
		t.Errorf("consensus-node must defer uid and runAsNonRoot to the image and pod, got %+v", main)
	}
}

// Without a writable /run, s6-overlay's /init fails under no_new_privs: "unable to mkdir /run/s6".
func TestBuildStatefulSet_ConsensusNodeWritableRun(t *testing.T) {
	r := newStsReconciler()
	sts, err := r.buildStatefulSet(context.Background(), capsuleWithImage("v0.72.1"))
	if err != nil {
		t.Fatalf("buildStatefulSet: %v", err)
	}
	spec := sts.Spec.Template.Spec

	var main *corev1.Container
	for i := range spec.Containers {
		if spec.Containers[i].Name == DefaultConsensusContainerName {
			main = &spec.Containers[i]
		}
	}
	if main == nil {
		t.Fatal("consensus-node container not found")
	}
	idx := slices.IndexFunc(main.VolumeMounts, func(m corev1.VolumeMount) bool { return m.MountPath == "/run" })
	if idx < 0 {
		t.Fatal("consensus-node must mount a writable /run")
	}
	if m := main.VolumeMounts[idx]; m.Name != VolumeTypeS6Run || m.ReadOnly {
		t.Errorf("/run mount must be writable %q, got %+v", VolumeTypeS6Run, m)
	}

	vi := slices.IndexFunc(spec.Volumes, func(v corev1.Volume) bool { return v.Name == VolumeTypeS6Run })
	if vi < 0 || spec.Volumes[vi].EmptyDir == nil || spec.Volumes[vi].EmptyDir.Medium != corev1.StorageMediumMemory {
		t.Errorf("%q must be a Memory emptyDir, got %+v", VolumeTypeS6Run, spec.Volumes)
	}
}

func TestBuildStatefulSet_CRSecurityContextOverrides(t *testing.T) {
	r := newStsReconciler()
	capsule := capsuleWithImage("v0.72.1")
	escalate := true
	uid := int64(1234)
	capsule.Spec.PodProperties.Containers.ConsensusNode.SecurityContext = &corev1.SecurityContext{
		AllowPrivilegeEscalation: &escalate,
		RunAsUser:                &uid,
		Capabilities:             &corev1.Capabilities{Add: []corev1.Capability{"NET_BIND_SERVICE"}},
	}
	localhost := "profiles/cn.json"
	capsule.Spec.PodProperties.SecurityContext = &corev1.PodSecurityContext{
		SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeLocalhost, LocalhostProfile: &localhost},
	}

	sts, err := r.buildStatefulSet(context.Background(), capsule)
	if err != nil {
		t.Fatalf("buildStatefulSet: %v", err)
	}

	sc := sts.Spec.Template.Spec.Containers[0].SecurityContext
	if sc.AllowPrivilegeEscalation == nil || !*sc.AllowPrivilegeEscalation {
		t.Errorf("CR allowPrivilegeEscalation=true must win, got %v", sc.AllowPrivilegeEscalation)
	}
	if sc.RunAsUser == nil || *sc.RunAsUser != uid {
		t.Errorf("CR runAsUser must win, got %v", sc.RunAsUser)
	}
	if !slices.Equal(sc.Capabilities.Add, []corev1.Capability{"NET_BIND_SERVICE"}) || !slices.Equal(sc.Capabilities.Drop, []corev1.Capability{"ALL"}) {
		t.Errorf("CR capabilities.add must merge with baseline drop [ALL], got %+v", sc.Capabilities)
	}

	psc := sts.Spec.Template.Spec.SecurityContext
	if psc.SeccompProfile.Type != corev1.SeccompProfileTypeLocalhost {
		t.Errorf("CR pod seccompProfile must win, got %+v", psc.SeccompProfile)
	}
	if psc.RunAsNonRoot == nil || !*psc.RunAsNonRoot {
		t.Errorf("baseline runAsNonRoot must fill the gap, got %v", psc.RunAsNonRoot)
	}
	if capsule.Spec.PodProperties.SecurityContext.RunAsNonRoot != nil {
		t.Error("hardening must not mutate the CR's pod securityContext")
	}
}

func TestHardenedSecurityContext(t *testing.T) {
	truth, root, sysAdmin := true, int64(0), corev1.Capability("CAP_SYS_ADMIN")
	emptyDrop := []corev1.Capability{}

	tests := []struct {
		name         string
		override     *corev1.SecurityContext
		wantDrop     []corev1.Capability
		wantNonRoot  *bool
		wantNilAPE   bool
		wantNilNonRt bool
	}{
		{name: "nil override yields baseline", override: nil, wantDrop: []corev1.Capability{"ALL"}, wantNilNonRt: true},
		{name: "privileged skips allowPrivilegeEscalation", override: &corev1.SecurityContext{Privileged: &truth}, wantNilAPE: true, wantDrop: []corev1.Capability{"ALL"}, wantNilNonRt: true},
		{name: "CAP_SYS_ADMIN skips allowPrivilegeEscalation", override: &corev1.SecurityContext{Capabilities: &corev1.Capabilities{Add: []corev1.Capability{sysAdmin}}}, wantNilAPE: true, wantDrop: []corev1.Capability{"ALL"}, wantNilNonRt: true},
		{name: "explicit empty drop is kept", override: &corev1.SecurityContext{Capabilities: &corev1.Capabilities{Drop: emptyDrop}}, wantDrop: emptyDrop, wantNilNonRt: true},
		{name: "explicit root opts out of runAsNonRoot", override: &corev1.SecurityContext{RunAsUser: &root}, wantDrop: []corev1.Capability{"ALL"}, wantNonRoot: new(bool)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc := hardenedSecurityContext(tt.override)
			if tt.wantNilAPE != (sc.AllowPrivilegeEscalation == nil) {
				t.Errorf("allowPrivilegeEscalation: got %v", sc.AllowPrivilegeEscalation)
			}
			if !tt.wantNilAPE && *sc.AllowPrivilegeEscalation {
				t.Error("allowPrivilegeEscalation must default to false")
			}
			if !slices.Equal(sc.Capabilities.Drop, tt.wantDrop) {
				t.Errorf("drop: got %v, want %v", sc.Capabilities.Drop, tt.wantDrop)
			}
			if tt.wantNilNonRt != (sc.RunAsNonRoot == nil) {
				t.Errorf("runAsNonRoot: got %v", sc.RunAsNonRoot)
			}
			if tt.wantNonRoot != nil && (sc.RunAsNonRoot == nil || *sc.RunAsNonRoot != *tt.wantNonRoot) {
				t.Errorf("runAsNonRoot: got %v, want %v", sc.RunAsNonRoot, *tt.wantNonRoot)
			}
		})
	}
}

func TestHardenedPodSecurityContext_ExplicitRootSkipsRunAsNonRoot(t *testing.T) {
	root := int64(0)
	psc := hardenedPodSecurityContext(&corev1.PodSecurityContext{RunAsUser: &root})
	if psc.RunAsNonRoot != nil {
		t.Errorf("runAsNonRoot must stay unset for explicit root, got %v", *psc.RunAsNonRoot)
	}
	if psc.SeccompProfile == nil || psc.SeccompProfile.Type != corev1.SeccompProfileTypeRuntimeDefault {
		t.Errorf("seccompProfile must default to RuntimeDefault, got %+v", psc.SeccompProfile)
	}
}

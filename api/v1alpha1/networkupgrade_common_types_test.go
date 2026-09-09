// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import "testing"

func sidecar(version string) *ImageComponentSpec {
	return &ImageComponentSpec{Version: version}
}

func TestUpgradeImagesSpec_Component(t *testing.T) {
	images := UpgradeImagesSpec{
		ConsensusNode:        ImageComponentSpec{Version: "cn"},
		RecordStreamUploader: sidecar("rsu"),
		EventStreamUploader:  sidecar("esu"),
		BlockStreamUploader:  sidecar("bsu"),
		BackupUploader:       sidecar("bu"),
		UC:                   sidecar("uc"),
	}

	cases := []struct {
		name        ComponentName
		wantPresent bool
		wantVersion string
	}{
		{ComponentConsensusNode, true, "cn"},
		{ComponentRecordStreamUploader, true, "rsu"},
		{ComponentEventStreamUploader, true, "esu"},
		{ComponentBlockStreamUploader, true, "bsu"},
		{ComponentBackupUploader, true, "bu"},
		{ComponentUC, true, "uc"},
		{ComponentName("bogus"), false, ""},
	}
	for _, tc := range cases {
		got, present := images.Component(tc.name)
		if present != tc.wantPresent {
			t.Errorf("%s: present = %v, want %v", tc.name, present, tc.wantPresent)
		}
		if present && got.Version != tc.wantVersion {
			t.Errorf("%s: version = %q, want %q", tc.name, got.Version, tc.wantVersion)
		}
	}
}

func TestUpgradeImagesSpec_Component_AbsentSidecars(t *testing.T) {
	// Only the required ConsensusNode is set; every optional sidecar is nil.
	images := UpgradeImagesSpec{ConsensusNode: ImageComponentSpec{Version: "cn"}}

	if _, present := images.Component(ComponentConsensusNode); !present {
		t.Error("consensusNode should always be present")
	}
	for _, name := range []ComponentName{
		ComponentRecordStreamUploader,
		ComponentEventStreamUploader,
		ComponentBlockStreamUploader,
		ComponentBackupUploader,
		ComponentUC,
	} {
		if _, present := images.Component(name); present {
			t.Errorf("%s should be absent when unset", name)
		}
	}
}

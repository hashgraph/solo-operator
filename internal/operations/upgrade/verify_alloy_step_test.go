// SPDX-License-Identifier: Apache-2.0

package upgrade

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

func TestVerifyAlloyStep_Name(t *testing.T) {
	step := NewVerifyAlloyStep(nil)
	if step.Name() != "VerifyAlloyChangeset" {
		t.Errorf("expected name 'VerifyAlloyChangeset', got %q", step.Name())
	}
}

func TestVerifyAlloyStep_Run_NoChangeset(t *testing.T) {
	step := NewVerifyAlloyStep(nil)
	op := &operatorv1alpha1.NetworkOperation{
		Spec: operatorv1alpha1.NetworkOperationSpec{
			NetworkUpgrade: &operatorv1alpha1.NetworkUpgradeSpec{},
		},
	}
	status := &UpgradeStatus{Phase: operatorv1alpha1.PhaseRunning}

	result := step.Run(context.Background(), op, status)

	if !result.HasSuccessfulCondition(StepConditionTypeVerifyChangeset, StepConditionReasonAlloy) {
		t.Error("expected successful condition when no changeset specified")
	}
}

func TestVerifyAlloyStep_Run_NilChangeset(t *testing.T) {
	step := NewVerifyAlloyStep(nil)
	op := &operatorv1alpha1.NetworkOperation{
		Spec: operatorv1alpha1.NetworkOperationSpec{
			NetworkUpgrade: &operatorv1alpha1.NetworkUpgradeSpec{
				Changesets: operatorv1alpha1.NetworkUpgradeChangesets{
					Alloy: nil,
				},
			},
		},
	}
	status := &UpgradeStatus{Phase: operatorv1alpha1.PhaseRunning}

	result := step.Run(context.Background(), op, status)

	if !result.HasSuccessfulCondition(StepConditionTypeVerifyChangeset, StepConditionReasonAlloy) {
		t.Error("expected successful condition when changeset is nil")
	}
}

func TestVerifyAlloyStep_Run_EmptyRepositories(t *testing.T) {
	step := NewVerifyAlloyStep(nil)
	op := &operatorv1alpha1.NetworkOperation{
		Spec: operatorv1alpha1.NetworkOperationSpec{
			NetworkUpgrade: &operatorv1alpha1.NetworkUpgradeSpec{
				Changesets: operatorv1alpha1.NetworkUpgradeChangesets{
					Alloy: &operatorv1alpha1.AlloyChangeset{
						SoftwareVersionSource: operatorv1alpha1.SoftwareVersionSource{
							ImageRepositories: []operatorv1alpha1.ImageRepository{},
						},
					},
				},
			},
		},
	}
	status := &UpgradeStatus{Phase: operatorv1alpha1.PhaseRunning}

	result := step.Run(context.Background(), op, status)

	if result.Phase != operatorv1alpha1.PhaseFailed {
		t.Error("expected failed phase when no image repositories provided")
	}
}

func TestVerifyAlloyStep_Run_ValidRepository(t *testing.T) {
	step := NewVerifyAlloyStep(nil)
	op := &operatorv1alpha1.NetworkOperation{
		Spec: operatorv1alpha1.NetworkOperationSpec{
			NetworkUpgrade: &operatorv1alpha1.NetworkUpgradeSpec{
				Changesets: operatorv1alpha1.NetworkUpgradeChangesets{
					Alloy: &operatorv1alpha1.AlloyChangeset{
						SoftwareVersionSource: operatorv1alpha1.SoftwareVersionSource{
							ImageRepositories: []operatorv1alpha1.ImageRepository{
								{
									Repository: "docker.io/grafana",
									ImageName:  "alloy",
									ImageTag:   "v1.8.1",
								},
							},
						},
					},
				},
			},
		},
	}
	status := &UpgradeStatus{Phase: operatorv1alpha1.PhaseRunning}

	result := step.Run(context.Background(), op, status)

	if !result.HasSuccessfulCondition(StepConditionTypeVerifyChangeset, StepConditionReasonAlloy) {
		t.Error("expected successful condition for valid repository")
	}
	if result.SoftwareSourceStatus == nil {
		t.Fatal("expected SoftwareSourceStatus to be set")
	}
	if len(result.SoftwareSourceStatus.Alloy) != 1 {
		t.Fatalf("expected 1 alloy status, got %d", len(result.SoftwareSourceStatus.Alloy))
	}
	alloyStatus := result.SoftwareSourceStatus.Alloy[0]
	if !alloyStatus.SelectedForDeployment {
		t.Error("expected alloy image to be selected for deployment")
	}
	if alloyStatus.Repository != "docker.io/grafana" {
		t.Errorf("expected repository 'docker.io/grafana', got %q", alloyStatus.Repository)
	}
	if alloyStatus.ImageName != "alloy" {
		t.Errorf("expected image name 'alloy', got %q", alloyStatus.ImageName)
	}
	if alloyStatus.ImageTag != "v1.8.1" {
		t.Errorf("expected image tag 'v1.8.1', got %q", alloyStatus.ImageTag)
	}
}

func TestVerifyAlloyStep_Run_InvalidRepository_MissingFields(t *testing.T) {
	step := NewVerifyAlloyStep(nil)

	tests := []struct {
		name string
		repo operatorv1alpha1.ImageRepository
	}{
		{
			name: "missing repository",
			repo: operatorv1alpha1.ImageRepository{ImageName: "alloy", ImageTag: "v1.8.1"},
		},
		{
			name: "missing image name",
			repo: operatorv1alpha1.ImageRepository{Repository: "docker.io/grafana", ImageTag: "v1.8.1"},
		},
		{
			name: "missing image tag",
			repo: operatorv1alpha1.ImageRepository{Repository: "docker.io/grafana", ImageName: "alloy"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			op := &operatorv1alpha1.NetworkOperation{
				Spec: operatorv1alpha1.NetworkOperationSpec{
					NetworkUpgrade: &operatorv1alpha1.NetworkUpgradeSpec{
						Changesets: operatorv1alpha1.NetworkUpgradeChangesets{
							Alloy: &operatorv1alpha1.AlloyChangeset{
								SoftwareVersionSource: operatorv1alpha1.SoftwareVersionSource{
									ImageRepositories: []operatorv1alpha1.ImageRepository{tc.repo},
								},
							},
						},
					},
				},
			}
			status := &UpgradeStatus{Phase: operatorv1alpha1.PhaseRunning}

			result := step.Run(context.Background(), op, status)

			if result.Phase != operatorv1alpha1.PhaseFailed {
				t.Error("expected failed phase for invalid repository")
			}
		})
	}
}

func TestVerifyAlloyStep_Run_SelectsFirstValid(t *testing.T) {
	step := NewVerifyAlloyStep(nil)
	op := &operatorv1alpha1.NetworkOperation{
		Spec: operatorv1alpha1.NetworkOperationSpec{
			NetworkUpgrade: &operatorv1alpha1.NetworkUpgradeSpec{
				Changesets: operatorv1alpha1.NetworkUpgradeChangesets{
					Alloy: &operatorv1alpha1.AlloyChangeset{
						SoftwareVersionSource: operatorv1alpha1.SoftwareVersionSource{
							ImageRepositories: []operatorv1alpha1.ImageRepository{
								{Repository: "first.io", ImageName: "alloy", ImageTag: "v1"},
								{Repository: "second.io", ImageName: "alloy", ImageTag: "v2"},
							},
						},
					},
				},
			},
		},
	}
	status := &UpgradeStatus{Phase: operatorv1alpha1.PhaseRunning}

	result := step.Run(context.Background(), op, status)

	if result.SoftwareSourceStatus.Alloy[0].Repository != "first.io" {
		t.Errorf("expected first valid repository to be selected, got %q", result.SoftwareSourceStatus.Alloy[0].Repository)
	}
}

func TestVerifyAlloyStep_IsSuccessful(t *testing.T) {
	step := NewVerifyAlloyStep(nil)
	op := &operatorv1alpha1.NetworkOperation{}

	status := &UpgradeStatus{
		Conditions: []metav1.Condition{
			{
				Type:   StepConditionTypeVerifyChangeset,
				Status: metav1.ConditionTrue,
				Reason: StepConditionReasonAlloy,
			},
		},
	}

	if !step.IsSuccessful(op, status) {
		t.Error("expected IsSuccessful to return true")
	}

	status.Conditions[0].Status = metav1.ConditionFalse
	if step.IsSuccessful(op, status) {
		t.Error("expected IsSuccessful to return false for failed condition")
	}
}

func TestVerifyAlloyStep_ShouldSkip(t *testing.T) {
	step := NewVerifyAlloyStep(nil)

	t.Run("skip when phase is failed", func(t *testing.T) {
		op := &operatorv1alpha1.NetworkOperation{}
		status := &UpgradeStatus{Phase: operatorv1alpha1.PhaseFailed}
		if !step.ShouldSkip(op, status) {
			t.Error("expected ShouldSkip when phase is failed")
		}
	})

	t.Run("skip when no changeset", func(t *testing.T) {
		op := &operatorv1alpha1.NetworkOperation{
			Spec: operatorv1alpha1.NetworkOperationSpec{
				NetworkUpgrade: &operatorv1alpha1.NetworkUpgradeSpec{},
			},
		}
		status := &UpgradeStatus{Phase: operatorv1alpha1.PhaseRunning}
		if !step.ShouldSkip(op, status) {
			t.Error("expected ShouldSkip when no changeset")
		}
	})

	t.Run("skip when already successful", func(t *testing.T) {
		op := &operatorv1alpha1.NetworkOperation{
			Spec: operatorv1alpha1.NetworkOperationSpec{
				NetworkUpgrade: &operatorv1alpha1.NetworkUpgradeSpec{
					Changesets: operatorv1alpha1.NetworkUpgradeChangesets{
						Alloy: &operatorv1alpha1.AlloyChangeset{},
					},
				},
			},
		}
		status := &UpgradeStatus{
			Phase: operatorv1alpha1.PhaseRunning,
			Conditions: []metav1.Condition{
				{
					Type:   StepConditionTypeVerifyChangeset,
					Status: metav1.ConditionTrue,
					Reason: StepConditionReasonAlloy,
				},
			},
		}
		if !step.ShouldSkip(op, status) {
			t.Error("expected ShouldSkip when already successful")
		}
	})

	t.Run("do not skip when changeset exists and not yet successful", func(t *testing.T) {
		op := &operatorv1alpha1.NetworkOperation{
			Spec: operatorv1alpha1.NetworkOperationSpec{
				NetworkUpgrade: &operatorv1alpha1.NetworkUpgradeSpec{
					Changesets: operatorv1alpha1.NetworkUpgradeChangesets{
						Alloy: &operatorv1alpha1.AlloyChangeset{},
					},
				},
			},
		}
		status := &UpgradeStatus{Phase: operatorv1alpha1.PhaseRunning}
		if step.ShouldSkip(op, status) {
			t.Error("expected not to skip when changeset exists and not yet verified")
		}
	})
}

func TestReverifyAlloyStep_Name(t *testing.T) {
	step := NewReverifyAlloyStep(nil)
	if step.Name() != "ReverifyAlloyChangeset" {
		t.Errorf("expected name 'ReverifyAlloyChangeset', got %q", step.Name())
	}
}

func TestReverifyAlloyStep_Run_NoChangeset(t *testing.T) {
	step := NewReverifyAlloyStep(nil)
	op := &operatorv1alpha1.NetworkOperation{
		Spec: operatorv1alpha1.NetworkOperationSpec{
			NetworkUpgrade: &operatorv1alpha1.NetworkUpgradeSpec{},
		},
	}
	status := &UpgradeStatus{Phase: operatorv1alpha1.PhaseRunning}

	result := step.Run(context.Background(), op, status)

	// Should return status unchanged when no changeset
	if result.Phase != operatorv1alpha1.PhaseRunning {
		t.Errorf("expected phase to remain Running, got %q", result.Phase)
	}
}

func TestReverifyAlloyStep_Run_AlreadySelected(t *testing.T) {
	step := NewReverifyAlloyStep(nil)
	op := &operatorv1alpha1.NetworkOperation{
		Spec: operatorv1alpha1.NetworkOperationSpec{
			NetworkUpgrade: &operatorv1alpha1.NetworkUpgradeSpec{
				Changesets: operatorv1alpha1.NetworkUpgradeChangesets{
					Alloy: &operatorv1alpha1.AlloyChangeset{
						SoftwareVersionSource: operatorv1alpha1.SoftwareVersionSource{
							ImageRepositories: []operatorv1alpha1.ImageRepository{
								{Repository: "docker.io/grafana", ImageName: "alloy", ImageTag: "v1.8.1"},
							},
						},
					},
				},
			},
		},
	}
	status := &UpgradeStatus{
		Phase: operatorv1alpha1.PhaseRunning,
		SoftwareSourceStatus: &operatorv1alpha1.SoftwareSourceStatus{
			Alloy: []operatorv1alpha1.ImageRepositoryStatus{
				{
					Repository:            "docker.io/grafana",
					ImageName:             "alloy",
					ImageTag:              "v1.8.1",
					SelectedForDeployment: true,
				},
			},
		},
	}

	result := step.Run(context.Background(), op, status)

	// Should return status unchanged when already selected
	if result.Phase != operatorv1alpha1.PhaseRunning {
		t.Errorf("expected phase to remain Running, got %q", result.Phase)
	}
}

func TestReverifyAlloyStep_ShouldSkip(t *testing.T) {
	step := NewReverifyAlloyStep(nil)

	t.Run("skip when failed", func(t *testing.T) {
		op := &operatorv1alpha1.NetworkOperation{}
		status := &UpgradeStatus{Phase: operatorv1alpha1.PhaseFailed}
		if !step.ShouldSkip(op, status) {
			t.Error("expected ShouldSkip when phase is failed")
		}
	})

	t.Run("skip when no changeset", func(t *testing.T) {
		op := &operatorv1alpha1.NetworkOperation{
			Spec: operatorv1alpha1.NetworkOperationSpec{
				NetworkUpgrade: &operatorv1alpha1.NetworkUpgradeSpec{},
			},
		}
		status := &UpgradeStatus{Phase: operatorv1alpha1.PhaseRunning}
		if !step.ShouldSkip(op, status) {
			t.Error("expected ShouldSkip when no changeset")
		}
	})
}

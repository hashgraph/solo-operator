with open("internal/controller/consensuscapsule_controller_test.go", "r") as f:
    text = f.read()

tests = """
type fakeS6 struct {
	up  bool
	err error
}

func (f *fakeS6) Start(ctx context.Context, capsule *operatorv1alpha1.ConsensusCapsule) error { return nil }
func (f *fakeS6) Stop(ctx context.Context, capsule *operatorv1alpha1.ConsensusCapsule) error { return nil }
func (f *fakeS6) IsUp(ctx context.Context, capsule *operatorv1alpha1.ConsensusCapsule) (bool, error) {
	return f.up, f.err
}

func TestFinalizeManualStopped(t *testing.T) {
	ctx := context.Background()
	capsule := newCapsuleForControllerTest()
	r := &ConsensusCapsuleReconciler{}
	s6 := &fakeS6{}
	r.S6 = s6

	// A cold idle Manual node (never live) still reaches Stopped promptly.
	t.Run("cold idle node", func(t *testing.T) {
		pod := newReadyPodWithPhase(capsule, corev1.PodRunning)
		var requeue bool
		s6.up = false
		s6.err = nil
		phase := r.finalizeManualStopped(ctx, capsule, pod, operatorv1alpha1.PhasePending, &requeue)
		if phase != operatorv1alpha1.PhaseStopped {
			t.Errorf("expected PhaseStopped, got %v", phase)
		}
	})

	// An IsUp error does not produce Stopped (keeps prior phase + requeues).
	t.Run("IsUp error", func(t *testing.T) {
		pod := newReadyPodWithPhase(capsule, corev1.PodRunning)
		var requeue bool
		s6.up = false
		s6.err = fmt.Errorf("exec failed")
		phase := r.finalizeManualStopped(ctx, capsule, pod, operatorv1alpha1.PhaseActive, &requeue)
		if phase != operatorv1alpha1.PhaseActive {
			t.Errorf("expected PhaseActive, got %v", phase)
		}
		if !requeue {
			t.Errorf("expected requeue=true")
		}
	})

	// UNKNOWN sustained >= debounce with s6 down -> Stopped
	t.Run("sustained UNKNOWN s6 down", func(t *testing.T) {
		pod := newReadyPodWithPhase(capsule, corev1.PodRunning)
		pod.Status.Conditions = append(pod.Status.Conditions, corev1.PodCondition{
			Type:               operatorv1alpha1.PodConditionPlatformStatus,
			LastTransitionTime: metav1.Time{Time: time.Now().Add(-20 * time.Second)},
		})
		var requeue bool
		s6.up = false
		s6.err = nil
		phase := r.finalizeManualStopped(ctx, capsule, pod, operatorv1alpha1.PhaseActive, &requeue)
		if phase != operatorv1alpha1.PhaseStopped {
			t.Errorf("expected PhaseStopped, got %v", phase)
		}
	})

	// UNKNOWN sustained >= debounce with s6 up -> Running
	t.Run("sustained UNKNOWN s6 up", func(t *testing.T) {
		pod := newReadyPodWithPhase(capsule, corev1.PodRunning)
		pod.Status.Conditions = append(pod.Status.Conditions, corev1.PodCondition{
			Type:               operatorv1alpha1.PodConditionPlatformStatus,
			LastTransitionTime: metav1.Time{Time: time.Now().Add(-20 * time.Second)},
		})
		var requeue bool
		s6.up = true
		s6.err = nil
		phase := r.finalizeManualStopped(ctx, capsule, pod, operatorv1alpha1.PhaseActive, &requeue)
		if phase != operatorv1alpha1.PhaseRunning {
			t.Errorf("expected PhaseRunning, got %v", phase)
		}
	})

	// UNKNOWN < debounce stays Active/Running — no Stopped flap
	t.Run("transient UNKNOWN", func(t *testing.T) {
		pod := newReadyPodWithPhase(capsule, corev1.PodRunning)
		pod.Status.Conditions = append(pod.Status.Conditions, corev1.PodCondition{
			Type:               operatorv1alpha1.PodConditionPlatformStatus,
			LastTransitionTime: metav1.Time{Time: time.Now().Add(-5 * time.Second)},
		})
		var requeue bool
		s6.up = false // shouldn't be called, but if it is it shouldn't hit Stopped
		s6.err = nil
		phase := r.finalizeManualStopped(ctx, capsule, pod, operatorv1alpha1.PhaseActive, &requeue)
		if phase != operatorv1alpha1.PhaseActive {
			t.Errorf("expected PhaseActive, got %v", phase)
		}
		if !requeue {
			t.Errorf("expected requeue=true for transient")
		}
	})
}
"""

with open("internal/controller/consensuscapsule_controller_test.go", "a") as f:
    f.write(tests)
print("success")

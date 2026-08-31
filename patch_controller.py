import re
with open("internal/controller/consensuscapsule_controller.go", "r") as f:
    text = f.read()

replacement_switch = """		switch pod.Status.Phase {
		case corev1.PodRunning:
			phase = r.rollupRunningPhase(capsule, pod, platformStatus, oldPhase, onUpdateRevision, &requeue)
		case corev1.PodSucceeded:
			phase = operatorv1alpha1.PhaseSucceeded
		case corev1.PodFailed:
			phase = operatorv1alpha1.PhaseFailed
		case corev1.PodPending:
			phase = operatorv1alpha1.PhasePending
		default:
			phase = operatorv1alpha1.PhaseUnknown
		}

		if phase == operatorv1alpha1.PhaseStopped {
			phase = r.finalizeManualStopped(ctx, capsule, &pod, oldPhase, &requeue)
		}
"""

pattern_switch = r"\t\tswitch pod\.Status\.Phase \{\n\t\tcase corev1\.PodRunning:\n\t\t\tphase = r\.rollupRunningPhase\(capsule, pod, platformStatus, oldPhase, onUpdateRevision, &requeue\)\n\t\tcase corev1\.PodSucceeded:\n\t\t\tphase = operatorv1alpha1\.PhaseSucceeded\n\t\tcase corev1\.PodFailed:\n\t\t\tphase = operatorv1alpha1\.PhaseFailed\n\t\tcase corev1\.PodPending:\n\t\t\tphase = operatorv1alpha1\.PhasePending\n\t\tdefault:\n\t\t\tphase = operatorv1alpha1\.PhaseUnknown\n\t\t\}"

if re.search(pattern_switch, text):
    text = re.sub(pattern_switch, replacement_switch, text)
else:
    print("pattern not found for switch")

additional_funcs = """
const manualStoppedDebounce = 15 * time.Second

func (r *ConsensusCapsuleReconciler) finalizeManualStopped(ctx context.Context, capsule *operatorv1alpha1.ConsensusCapsule, pod *corev1.Pod, oldPhase operatorv1alpha1.Phase, requeue *bool) operatorv1alpha1.Phase {
	if oldPhase == operatorv1alpha1.PhaseStopped {
		return operatorv1alpha1.PhaseStopped
	}

	var unknownSince time.Time
	cond := findPlatformStatusCondition(pod)
	if cond != nil {
		unknownSince = cond.LastTransitionTime.Time
	}

	wasLive := oldPhase == operatorv1alpha1.PhaseActive || oldPhase == operatorv1alpha1.PhaseRunning

	if wasLive && time.Since(unknownSince) < manualStoppedDebounce {
		*requeue = true
		if oldPhase == operatorv1alpha1.PhaseActive {
			return operatorv1alpha1.PhaseActive
		}
		return operatorv1alpha1.PhaseRunning
	}

	up, err := r.S6.IsUp(ctx, capsule)
	switch {
	case err != nil:
		*requeue = true
		if wasLive {
			if oldPhase == operatorv1alpha1.PhaseActive {
				return operatorv1alpha1.PhaseActive
			}
			return operatorv1alpha1.PhaseRunning
		}
		return oldPhase
	case up:
		if r.Recorder != nil {
			r.Recorder.Event(capsule, corev1.EventTypeNormal, operatorv1alpha1.EventConsensusNodeActive.String(), "platform metrics unavailable but consensus process running")
		}
		return operatorv1alpha1.PhaseRunning
	default:
		return operatorv1alpha1.PhaseStopped
	}
}
"""

text = text + additional_funcs

with open("internal/controller/consensuscapsule_controller.go", "w") as f:
    f.write(text)
    print("success")

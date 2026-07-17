// SPDX-License-Identifier: Apache-2.0

package common

import (
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/events"
)

func TestEventf_EmitsEventAndPairedLog(t *testing.T) {
	buf := captureLogs(t)
	rec := events.NewFakeRecorder(1)

	ctx := ReconcileContext(context.Background(), "networkupgradeexecute", "NetworkUpgradeExecute",
		&metav1.PartialObjectMetadata{ObjectMeta: metav1.ObjectMeta{Namespace: "solo", Name: "up-1"}})
	obj := &corev1.Pod{}

	Eventf(ctx, rec, obj, corev1.EventTypeWarning, "UpgradeFailed", "Execute", "daemon reported %s", "boom")

	// Paired log line: matching reason=, formatted message, inherited fields.
	m := decodeLine(t, buf)
	if got, _ := m["reason"].(string); got != "UpgradeFailed" {
		t.Errorf("reason = %q, want %q", got, "UpgradeFailed")
	}
	if got, _ := m["level"].(string); got != "warn" {
		t.Errorf("level = %q, want warn (Warning event)", got)
	}
	if got, _ := m["message"].(string); got != "daemon reported boom" {
		t.Errorf("message = %q, want %q", got, "daemon reported boom")
	}
	if got, _ := m["name"].(string); got != "up-1" {
		t.Errorf("name = %q, want up-1 (inherited)", got)
	}

	// K8s event carries the same reason and message.
	select {
	case e := <-rec.Events:
		if !strings.Contains(e, "UpgradeFailed") || !strings.Contains(e, "daemon reported boom") {
			t.Errorf("event = %q, want reason+message", e)
		}
	default:
		t.Error("no event recorded")
	}
}

func TestEventf_NormalEventLogsAtInfo(t *testing.T) {
	buf := captureLogs(t)
	rec := events.NewFakeRecorder(1)

	Eventf(context.Background(), rec, &corev1.Pod{}, corev1.EventTypeNormal, "CapsulePatched", "Execute", "patched")

	m := decodeLine(t, buf)
	if got, _ := m["level"].(string); got != "info" {
		t.Errorf("level = %q, want info (Normal event)", got)
	}
}

func TestEventf_NilRecorderStillLogs(t *testing.T) {
	buf := captureLogs(t)

	Eventf(context.Background(), nil, &corev1.Pod{}, corev1.EventTypeNormal, "CapsulePatched", "Execute", "patched")

	m := decodeLine(t, buf)
	if got, _ := m["reason"].(string); got != "CapsulePatched" {
		t.Errorf("reason = %q, want CapsulePatched (log emitted even without recorder)", got)
	}
}

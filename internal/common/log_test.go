// SPDX-License-Identifier: Apache-2.0

package common

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/automa-saga/logx"
	"github.com/rs/zerolog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// captureLogs swaps the global logx logger for a JSON logger writing to buf for
// the duration of the test, then restores the original.
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	orig := logx.As()
	origLevel := zerolog.GlobalLevel()
	buf := &bytes.Buffer{}
	logx.SetLogger(zerolog.New(buf).Level(zerolog.DebugLevel))
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	t.Cleanup(func() {
		logx.SetLogger(*orig)
		zerolog.SetGlobalLevel(origLevel)
	})
	return buf
}

func decodeLine(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("log line is not valid JSON: %v (%q)", err, buf.String())
	}
	return m
}

func TestInfo_StampsReconcileFieldsAndReason(t *testing.T) {
	buf := captureLogs(t)

	ctx := WithLogFields(context.Background(), LogFields{
		Controller:  "helmcapsule",
		Kind:        "HelmCapsule",
		Namespace:   "solo",
		Name:        "hc-1",
		ReconcileID: "rid-123",
	})
	Info(ctx, "TestReason").Msg("hello")

	m := decodeLine(t, buf)
	for k, want := range map[string]string{
		"controller":  "helmcapsule",
		"kind":        "HelmCapsule",
		"namespace":   "solo",
		"name":        "hc-1",
		"reconcileID": "rid-123",
		"reason":      "TestReason",
		"message":     "hello",
	} {
		if got, _ := m[k].(string); got != want {
			t.Errorf("field %q = %q, want %q", k, got, want)
		}
	}
	if _, present := m["operation_id"]; present {
		t.Errorf("operation_id should be omitted when unset, got %v", m["operation_id"])
	}
}

func TestInfo_IncludesOperationIDWhenSet(t *testing.T) {
	buf := captureLogs(t)

	ctx := WithLogFields(context.Background(), LogFields{
		Controller:  "networkupgradeexecute",
		Kind:        "NetworkUpgradeExecute",
		OperationID: "op-42",
	})
	Info(ctx, "TestReason").Msg("hi")

	m := decodeLine(t, buf)
	if got, _ := m["operation_id"].(string); got != "op-42" {
		t.Errorf("operation_id = %q, want %q", got, "op-42")
	}
}

// opIDObj is a minimal metav1.Object that also implements OperationIDer.
type opIDObj struct {
	metav1.ObjectMeta
	opID string
}

func (o *opIDObj) GetOperationID() string { return o.opID }

func TestReconcileContext_StampsOperationIDViaInterface(t *testing.T) {
	buf := captureLogs(t)

	obj := &opIDObj{
		ObjectMeta: metav1.ObjectMeta{Namespace: "solo", Name: "up-1"},
		opID:       "op-99",
	}
	ctx := ReconcileContext(context.Background(), "networkupgradeexecute", "NetworkUpgradeExecute", obj)
	Info(ctx, "TestReason").Msg("hi")

	m := decodeLine(t, buf)
	if got, _ := m["operation_id"].(string); got != "op-99" {
		t.Errorf("operation_id = %q, want %q (should be extracted via OperationIDer)", got, "op-99")
	}
	if got, _ := m["name"].(string); got != "up-1" {
		t.Errorf("name = %q, want %q", got, "up-1")
	}
}

func TestReconcileContext_OmitsOperationIDForPlainObject(t *testing.T) {
	buf := captureLogs(t)

	// A plain metav1.Object that does NOT implement OperationIDer.
	obj := &metav1.PartialObjectMetadata{
		ObjectMeta: metav1.ObjectMeta{Namespace: "solo", Name: "cc-1"},
	}
	ctx := ReconcileContext(context.Background(), "consensuscapsule", "ConsensusCapsule", obj)
	Info(ctx, "TestReason").Msg("hi")

	m := decodeLine(t, buf)
	if _, present := m["operation_id"]; present {
		t.Errorf("operation_id should be omitted for a CR without one, got %v", m["operation_id"])
	}
}

func TestInfo_NoReconcileFieldsOutsideReconcile(t *testing.T) {
	buf := captureLogs(t)

	// No WithLogFields on the context — mirrors the uc daemon path.
	Info(context.Background(), "TestReason").Msg("bare")

	m := decodeLine(t, buf)
	if got, _ := m["reason"].(string); got != "TestReason" {
		t.Errorf("reason = %q, want %q", got, "TestReason")
	}
	for _, k := range []string{"controller", "kind", "namespace", "name", "reconcileID", "operation_id"} {
		if _, present := m[k]; present {
			t.Errorf("field %q should be absent outside a reconcile, got %v", k, m[k])
		}
	}
}

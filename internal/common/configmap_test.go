// SPDX-License-Identifier: Apache-2.0

package common

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

var testGVK = schema.GroupVersionKind{
	Group:   "operator.solo.hiero.org",
	Version: "v1alpha1",
	Kind:    "NodeSettings",
}

var _ = runtime.Object(&fakeCaller{})

type fakeCaller struct {
	metav1.ObjectMeta
	metav1.TypeMeta
}

func (f *fakeCaller) GetObjectKind() schema.ObjectKind { return &f.TypeMeta }
func (f *fakeCaller) DeepCopyObject() runtime.Object   { return f }

func TestConfigMapOwnerConflict(t *testing.T) {
	callerUID := types.UID("uid-1")
	isController := true

	tests := []struct {
		name    string
		cm      *corev1.ConfigMap
		wantMsg bool
	}{
		{
			name:    "no owner",
			cm:      &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "my-settings"}},
			wantMsg: false,
		},
		{
			name: "same kind and UID",
			cm: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
				Name: "my-settings",
				OwnerReferences: []metav1.OwnerReference{{
					Kind: "NodeSettings", Name: "my-settings", UID: "uid-1", Controller: &isController,
				}},
			}},
			wantMsg: false,
		},
		{
			name: "different kind",
			cm: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
				Name: "my-settings",
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: "operator.solo.hiero.org/v1alpha1",
					Kind:       "Log4j2Config", Name: "my-settings", UID: "uid-2", Controller: &isController,
				}},
			}},
			wantMsg: true,
		},
		{
			name: "same kind different UID — allowed for upgrade takeover",
			cm: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
				Name: "my-settings",
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: "operator.solo.hiero.org/v1alpha1",
					Kind:       "NodeSettings", Name: "other", UID: "uid-99", Controller: &isController,
				}},
			}},
			wantMsg: false,
		},
		{
			name: "same kind different APIVersion",
			cm: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
				Name: "my-settings",
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: "other.group/v1beta1",
					Kind:       "NodeSettings", Name: "other", UID: "uid-99", Controller: &isController,
				}},
			}},
			wantMsg: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := ConfigMapOwnerConflict(tt.cm, callerUID, testGVK)
			if tt.wantMsg && msg == "" {
				t.Error("expected conflict message, got empty")
			}
			if !tt.wantMsg && msg != "" {
				t.Errorf("expected no conflict, got %q", msg)
			}
		})
	}
}

func TestConfigMapOwnerConflict_ExplicitGVK(t *testing.T) {
	isController := true
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Name: "my-settings",
		OwnerReferences: []metav1.OwnerReference{{
			APIVersion: "operator.solo.hiero.org/v1alpha1",
			Kind:       "NodeSettings", Name: "other", UID: "uid-99", Controller: &isController,
		}},
	}}
	if msg := ConfigMapOwnerConflict(cm, types.UID("uid-1"), testGVK); msg != "" {
		t.Errorf("expected same-kind takeover with explicit GVK, got conflict: %s", msg)
	}
}

func TestConfigMapSuperseded(t *testing.T) {
	now := time.Date(2026, 8, 11, 4, 40, 51, 0, time.UTC)
	earlier := now.Add(-time.Hour)
	later := now.Add(time.Hour)
	isController := true

	makeCallerAt := func(uid string, ts time.Time) *fakeCaller {
		return &fakeCaller{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "caller",
				UID:               types.UID(uid),
				CreationTimestamp: metav1.NewTime(ts),
			},
			TypeMeta: metav1.TypeMeta{Kind: "NodeSettings", APIVersion: "operator.solo.hiero.org/v1alpha1"},
		}
	}

	tests := []struct {
		name   string
		cm     *corev1.ConfigMap
		caller *fakeCaller
		want   bool
	}{
		{
			name:   "no controller ref",
			cm:     &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cm"}},
			caller: makeCallerAt("uid-1", now),
			want:   false,
		},
		{
			name: "same UID",
			cm: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
				Name:            "cm",
				Annotations:     map[string]string{AnnotationOwnerCreatedAt: now.Format(time.RFC3339Nano)},
				OwnerReferences: []metav1.OwnerReference{{UID: "uid-1", Controller: &isController}},
			}},
			caller: makeCallerAt("uid-1", earlier),
			want:   false,
		},
		{
			name: "no annotation",
			cm: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
				Name:            "cm",
				OwnerReferences: []metav1.OwnerReference{{UID: "uid-2", Controller: &isController}},
			}},
			caller: makeCallerAt("uid-1", earlier),
			want:   false,
		},
		{
			name: "caller is older — superseded",
			cm: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
				Name:            "cm",
				Annotations:     map[string]string{AnnotationOwnerCreatedAt: now.Format(time.RFC3339Nano)},
				OwnerReferences: []metav1.OwnerReference{{UID: "uid-2", Controller: &isController}},
			}},
			caller: makeCallerAt("uid-1", earlier),
			want:   true,
		},
		{
			name: "caller is newer — not superseded",
			cm: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
				Name:            "cm",
				Annotations:     map[string]string{AnnotationOwnerCreatedAt: now.Format(time.RFC3339Nano)},
				OwnerReferences: []metav1.OwnerReference{{UID: "uid-2", Controller: &isController}},
			}},
			caller: makeCallerAt("uid-1", later),
			want:   false,
		},
		{
			name: "same creation time — superseded (not strictly after)",
			cm: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
				Name:            "cm",
				Annotations:     map[string]string{AnnotationOwnerCreatedAt: now.Format(time.RFC3339Nano)},
				OwnerReferences: []metav1.OwnerReference{{UID: "uid-2", Controller: &isController}},
			}},
			caller: makeCallerAt("uid-1", now),
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConfigMapSuperseded(tt.cm, tt.caller)
			if got != tt.want {
				t.Errorf("ConfigMapSuperseded() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStampOwnerCreatedAt(t *testing.T) {
	ts := time.Date(2026, 8, 11, 4, 40, 51, 0, time.UTC)
	caller := &fakeCaller{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test",
			CreationTimestamp: metav1.NewTime(ts),
		},
	}
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cm"}}

	StampOwnerCreatedAt(cm, caller)

	ann := cm.GetAnnotations()
	if ann == nil {
		t.Fatal("expected annotations, got nil")
	}
	want := "2026-08-11T04:40:51Z"
	if got := ann[AnnotationOwnerCreatedAt]; got != want {
		t.Errorf("annotation = %q, want %q", got, want)
	}
}

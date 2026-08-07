// SPDX-License-Identifier: Apache-2.0

package common

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

type fakeCaller struct {
	metav1.ObjectMeta
	metav1.TypeMeta
}

func (f *fakeCaller) GetObjectKind() schema.ObjectKind { return &f.TypeMeta }
func (f *fakeCaller) DeepCopyObject() runtime.Object   { return f }

func TestConfigMapOwnerConflict(t *testing.T) {
	caller := &fakeCaller{
		ObjectMeta: metav1.ObjectMeta{Name: "my-settings", UID: types.UID("uid-1")},
		TypeMeta:   metav1.TypeMeta{Kind: "NodeSettings", APIVersion: "v1alpha1"},
	}
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
					Kind: "Log4j2Config", Name: "my-settings", UID: "uid-2", Controller: &isController,
				}},
			}},
			wantMsg: true,
		},
		{
			name: "same kind different UID",
			cm: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
				Name: "my-settings",
				OwnerReferences: []metav1.OwnerReference{{
					Kind: "NodeSettings", Name: "other", UID: "uid-99", Controller: &isController,
				}},
			}},
			wantMsg: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := ConfigMapOwnerConflict(tt.cm, caller)
			if tt.wantMsg && msg == "" {
				t.Error("expected conflict message, got empty")
			}
			if !tt.wantMsg && msg != "" {
				t.Errorf("expected no conflict, got %q", msg)
			}
		})
	}
}

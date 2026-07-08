// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
)

type configCREntry struct {
	nameSuffix string
	content    string
}

type configCRKind struct {
	kind    string
	newList func() client.ObjectList
	// getLabels returns the labels of the i-th item in the list.
	getLabels func(list client.ObjectList, i int) map[string]string
	listLen   func(list client.ObjectList) int
	build     func(om metav1.ObjectMeta, orbit, content string) client.Object
}

var allConfigCRKinds = []configCRKind{
	{
		kind:    "ApiPermissionProperties",
		newList: func() client.ObjectList { return &operatorv1alpha1.ApiPermissionPropertiesList{} },
		getLabels: func(l client.ObjectList, i int) map[string]string {
			return l.(*operatorv1alpha1.ApiPermissionPropertiesList).Items[i].Labels
		},
		listLen: func(l client.ObjectList) int { return len(l.(*operatorv1alpha1.ApiPermissionPropertiesList).Items) },
		build: func(om metav1.ObjectMeta, orbit, content string) client.Object {
			return &operatorv1alpha1.ApiPermissionProperties{ObjectMeta: om, Spec: operatorv1alpha1.ApiPermissionPropertiesSpec{Content: content, Orbit: orbit}}
		},
	},
	{
		kind:    "ApplicationOverrideProperties",
		newList: func() client.ObjectList { return &operatorv1alpha1.ApplicationOverridePropertiesList{} },
		getLabels: func(l client.ObjectList, i int) map[string]string {
			return l.(*operatorv1alpha1.ApplicationOverridePropertiesList).Items[i].Labels
		},
		listLen: func(l client.ObjectList) int {
			return len(l.(*operatorv1alpha1.ApplicationOverridePropertiesList).Items)
		},
		build: func(om metav1.ObjectMeta, orbit, content string) client.Object {
			return &operatorv1alpha1.ApplicationOverrideProperties{ObjectMeta: om, Spec: operatorv1alpha1.ApplicationOverridePropertiesSpec{Content: content, Orbit: orbit}}
		},
	},
	{
		kind:    "ApplicationProperties",
		newList: func() client.ObjectList { return &operatorv1alpha1.ApplicationPropertiesList{} },
		getLabels: func(l client.ObjectList, i int) map[string]string {
			return l.(*operatorv1alpha1.ApplicationPropertiesList).Items[i].Labels
		},
		listLen: func(l client.ObjectList) int { return len(l.(*operatorv1alpha1.ApplicationPropertiesList).Items) },
		build: func(om metav1.ObjectMeta, orbit, content string) client.Object {
			return &operatorv1alpha1.ApplicationProperties{ObjectMeta: om, Spec: operatorv1alpha1.ApplicationPropertiesSpec{Content: content, Orbit: orbit}}
		},
	},
	{
		kind:    "BootstrapProperties",
		newList: func() client.ObjectList { return &operatorv1alpha1.BootstrapPropertiesList{} },
		getLabels: func(l client.ObjectList, i int) map[string]string {
			return l.(*operatorv1alpha1.BootstrapPropertiesList).Items[i].Labels
		},
		listLen: func(l client.ObjectList) int { return len(l.(*operatorv1alpha1.BootstrapPropertiesList).Items) },
		build: func(om metav1.ObjectMeta, orbit, content string) client.Object {
			return &operatorv1alpha1.BootstrapProperties{ObjectMeta: om, Spec: operatorv1alpha1.BootstrapPropertiesSpec{Content: content, Orbit: orbit}}
		},
	},
	{
		kind:    "FeeSchedules",
		newList: func() client.ObjectList { return &operatorv1alpha1.FeeSchedulesList{} },
		getLabels: func(l client.ObjectList, i int) map[string]string {
			return l.(*operatorv1alpha1.FeeSchedulesList).Items[i].Labels
		},
		listLen: func(l client.ObjectList) int { return len(l.(*operatorv1alpha1.FeeSchedulesList).Items) },
		build: func(om metav1.ObjectMeta, orbit, content string) client.Object {
			return &operatorv1alpha1.FeeSchedules{ObjectMeta: om, Spec: operatorv1alpha1.FeeSchedulesSpec{Content: content, Orbit: orbit}}
		},
	},
	{
		kind:    "Log4j2Config",
		newList: func() client.ObjectList { return &operatorv1alpha1.Log4j2ConfigList{} },
		getLabels: func(l client.ObjectList, i int) map[string]string {
			return l.(*operatorv1alpha1.Log4j2ConfigList).Items[i].Labels
		},
		listLen: func(l client.ObjectList) int { return len(l.(*operatorv1alpha1.Log4j2ConfigList).Items) },
		build: func(om metav1.ObjectMeta, orbit, content string) client.Object {
			return &operatorv1alpha1.Log4j2Config{ObjectMeta: om, Spec: operatorv1alpha1.Log4j2ConfigSpec{Content: content, Orbit: orbit}}
		},
	},
	{
		kind:    "NodePropertiesConfig",
		newList: func() client.ObjectList { return &operatorv1alpha1.NodePropertiesConfigList{} },
		getLabels: func(l client.ObjectList, i int) map[string]string {
			return l.(*operatorv1alpha1.NodePropertiesConfigList).Items[i].Labels
		},
		listLen: func(l client.ObjectList) int { return len(l.(*operatorv1alpha1.NodePropertiesConfigList).Items) },
		build: func(om metav1.ObjectMeta, orbit, content string) client.Object {
			return &operatorv1alpha1.NodePropertiesConfig{ObjectMeta: om, Spec: operatorv1alpha1.NodePropertiesConfigSpec{Content: content, Orbit: orbit}}
		},
	},
	{
		kind:    "NodeSettings",
		newList: func() client.ObjectList { return &operatorv1alpha1.NodeSettingsList{} },
		getLabels: func(l client.ObjectList, i int) map[string]string {
			return l.(*operatorv1alpha1.NodeSettingsList).Items[i].Labels
		},
		listLen: func(l client.ObjectList) int { return len(l.(*operatorv1alpha1.NodeSettingsList).Items) },
		build: func(om metav1.ObjectMeta, orbit, content string) client.Object {
			return &operatorv1alpha1.NodeSettings{ObjectMeta: om, Spec: operatorv1alpha1.NodeSettingsSpec{Content: content, Orbit: orbit}}
		},
	},
	{
		kind:    "SimpleFeesSchedules",
		newList: func() client.ObjectList { return &operatorv1alpha1.SimpleFeesSchedulesList{} },
		getLabels: func(l client.ObjectList, i int) map[string]string {
			return l.(*operatorv1alpha1.SimpleFeesSchedulesList).Items[i].Labels
		},
		listLen: func(l client.ObjectList) int { return len(l.(*operatorv1alpha1.SimpleFeesSchedulesList).Items) },
		build: func(om metav1.ObjectMeta, orbit, content string) client.Object {
			return &operatorv1alpha1.SimpleFeesSchedules{ObjectMeta: om, Spec: operatorv1alpha1.SimpleFeesSchedulesSpec{Content: content, Orbit: orbit}}
		},
	},
	{
		kind:    "ThrottlesConfig",
		newList: func() client.ObjectList { return &operatorv1alpha1.ThrottlesConfigList{} },
		getLabels: func(l client.ObjectList, i int) map[string]string {
			return l.(*operatorv1alpha1.ThrottlesConfigList).Items[i].Labels
		},
		listLen: func(l client.ObjectList) int { return len(l.(*operatorv1alpha1.ThrottlesConfigList).Items) },
		build: func(om metav1.ObjectMeta, orbit, content string) client.Object {
			return &operatorv1alpha1.ThrottlesConfig{ObjectMeta: om, Spec: operatorv1alpha1.ThrottlesConfigSpec{Content: content, Orbit: orbit}}
		},
	},
}

type configCRDedupeResult int

const (
	configCRNotFound    configCRDedupeResult = iota // no CR of this kind exists → create
	configCRMatchesOp                               // CR with matching operation-id found → skip
	configCRNoLabel                                 // CR exists but has no operation-id label → leave alone
	configCRDifferentOp                             // CR exists with a different operation-id → create new
)

func checkConfigCRExists(ctx context.Context, c client.Client, namespace, operationID string, kind configCRKind) (configCRDedupeResult, error) {
	list := kind.newList()
	if err := c.List(ctx, list, client.InNamespace(namespace)); err != nil {
		return configCRNotFound, fmt.Errorf("listing %s: %w", kind.kind, err)
	}
	n := kind.listLen(list)
	if n == 0 {
		return configCRNotFound, nil
	}
	// Scan all items: a CR with the matching operation-id takes precedence over
	// one without a label, so we must not short-circuit on the first unlabeled CR.
	hasNoLabel := false
	for i := range n {
		label := kind.getLabels(list, i)[common.LabelOperationID]
		if label == operationID {
			return configCRMatchesOp, nil
		}
		if label == "" {
			hasNoLabel = true
		}
	}
	if hasNoLabel {
		return configCRNoLabel, nil
	}
	return configCRDifferentOp, nil
}

func (r *NetworkUpgradeExecuteReconciler) reconcileConfigCRs(
	ctx context.Context,
	execute *operatorv1alpha1.NetworkUpgradeExecute,
	entries map[string]configCREntry,
) (bool, error) {
	logger := log.FromContext(ctx)
	operationID := execute.Spec.OperationID
	orbit := execute.Spec.Orbit
	namespace := execute.Namespace

	for _, kind := range allConfigCRKinds {
		entry, ok := entries[kind.kind]
		if !ok || entry.content == "" {
			continue
		}

		result, err := checkConfigCRExists(ctx, r.Client, namespace, operationID, kind)
		if err != nil {
			return false, err
		}

		switch result {
		case configCRMatchesOp:
			logger.V(1).Info("config CR already applied for this operation", "kind", kind.kind, "operationId", operationID)
			continue

		case configCRNoLabel:
			logger.Info("config CR exists without operation-id label (user-managed); leaving untouched",
				"kind", kind.kind, "operationId", operationID)
			r.eventf(execute, corev1.EventTypeWarning, operatorv1alpha1.EventConfigCRReconciled,
				"%s: existing user-managed CR left untouched for operationId %q", kind.kind, operationID)
			continue

		case configCRNotFound, configCRDifferentOp:
			crName := strings.ToLower(operationID) + "-" + entry.nameSuffix
			if err := r.createConfigCR(ctx, execute, kind, crName, orbit, operationID, entry.content); err != nil {
				return false, err
			}
			r.eventf(execute, corev1.EventTypeNormal, operatorv1alpha1.EventConfigCRReconciled,
				"%s: created %s for operationId %q", kind.kind, crName, operationID)
		}
	}
	return true, nil
}

// AlreadyExists is treated as success (idempotent re-entry across reconcile ticks)
func (r *NetworkUpgradeExecuteReconciler) createConfigCR(
	ctx context.Context,
	execute *operatorv1alpha1.NetworkUpgradeExecute,
	kind configCRKind,
	name, orbit, operationID, content string,
) error {
	om := metav1.ObjectMeta{
		Name:      name,
		Namespace: execute.Namespace,
		Labels:    map[string]string{common.LabelOperationID: operationID},
	}
	obj := kind.build(om, orbit, content)
	if err := r.Create(ctx, obj); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return nil
		}
		return fmt.Errorf("creating %s %s: %w", kind.kind, name, err)
	}
	return nil
}

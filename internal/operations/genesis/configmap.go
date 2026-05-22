// SPDX-License-Identifier: Apache-2.0

package genesis

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/hashgraph/solo-operator/internal/common"
)

func UpsertConfigMap(ctx context.Context, c client.Client, namespace, name, orbitName, operationID string, data map[string]string, binaryData map[string][]byte) error {
	labels := common.StandardLabels(name, orbitName)
	if operationID != "" {
		labels[common.LabelOperationID] = operationID
	}

	cm := &corev1.ConfigMap{}
	err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, cm)
	if apierrors.IsNotFound(err) {
		cm = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Labels:    labels,
			},
			Data:       data,
			BinaryData: binaryData,
		}
		return c.Create(ctx, cm)
	}
	if err != nil {
		return err
	}
	cm.Data = data
	cm.BinaryData = binaryData
	cm.Labels = labels
	return c.Update(ctx, cm)
}

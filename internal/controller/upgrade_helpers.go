// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

func findOneByOperationID[T any, TList client.ObjectList](
	ctx context.Context,
	c client.Client,
	namespace, operationID, kind string,
	newList func() TList,
	items func(TList) []T,
) (*T, error) {
	list := newList()
	if err := c.List(ctx, list,
		client.InNamespace(namespace),
		client.MatchingFields{fieldOperationID: operationID},
	); err != nil {
		return nil, fmt.Errorf("listing %s: %w", kind, err)
	}
	its := items(list)
	if len(its) == 0 {
		return nil, nil
	}
	if len(its) > 1 {
		return nil, fmt.Errorf("expected at most one %s for operation %q, found %d", kind, operationID, len(its))
	}
	return &its[0], nil
}

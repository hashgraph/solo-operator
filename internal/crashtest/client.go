// SPDX-License-Identifier: Apache-2.0

// Package crashtest simulates an operator process dying mid-reconcile.
package crashtest

import (
	"context"
	"errors"
	"sync/atomic"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

var ErrDead = errors.New("simulated operator crash")

// Wrap lets the write that satisfies dieAfter land, then fails every later write.
func Wrap(base client.WithWatch, dead *atomic.Bool, dieAfter func(client.Object) bool) client.WithWatch {
	do := func(obj client.Object, write func() error) error {
		if dead.Load() {
			return ErrDead
		}
		err := write()
		if err == nil && dieAfter(obj) {
			dead.Store(true)
		}
		return err
	}
	return interceptor.NewClient(base, interceptor.Funcs{
		Create: func(ctx context.Context, c client.WithWatch, obj client.Object, o ...client.CreateOption) error {
			return do(obj, func() error { return c.Create(ctx, obj, o...) })
		},
		Update: func(ctx context.Context, c client.WithWatch, obj client.Object, o ...client.UpdateOption) error {
			return do(obj, func() error { return c.Update(ctx, obj, o...) })
		},
		Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, p client.Patch, o ...client.PatchOption) error {
			return do(obj, func() error { return c.Patch(ctx, obj, p, o...) })
		},
		Delete: func(ctx context.Context, c client.WithWatch, obj client.Object, o ...client.DeleteOption) error {
			return do(obj, func() error { return c.Delete(ctx, obj, o...) })
		},
		SubResourceUpdate: func(ctx context.Context, c client.Client, sub string, obj client.Object, o ...client.SubResourceUpdateOption) error {
			return do(obj, func() error { return c.SubResource(sub).Update(ctx, obj, o...) })
		},
		SubResourcePatch: func(ctx context.Context, c client.Client, sub string, obj client.Object, p client.Patch, o ...client.SubResourcePatchOption) error {
			return do(obj, func() error { return c.SubResource(sub).Patch(ctx, obj, p, o...) })
		},
	})
}

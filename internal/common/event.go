// SPDX-License-Identifier: Apache-2.0

package common

import (
	"context"
	"fmt"

	"github.com/automa-saga/errx"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
)

// Eventf records a K8s event and a paired log line; Warning events log at Warn.
// Normal events log at Debug: they fire on every reconcile (e.g. skipped optional
// sidecars) and would otherwise flood INFO, while the K8s Event is still recorded
// for `kubectl describe`.
func Eventf(ctx context.Context, recorder events.EventRecorder, obj runtime.Object, eventtype string, reason errx.Reason, action, format string, args ...any) {
	open := Debug
	if eventtype == corev1.EventTypeWarning {
		open = Warn
	}
	open(ctx, reason).Msg(fmt.Sprintf(format, args...))

	if recorder != nil {
		recorder.Eventf(obj, nil, eventtype, reason.String(), action, format, args...)
	}
}

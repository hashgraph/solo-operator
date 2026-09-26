// SPDX-License-Identifier: Apache-2.0

package common

import (
	"context"
	"fmt"

	"github.com/automa-saga/errx"
	"github.com/rs/zerolog"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
)

// Eventf records a K8s event and a paired log line; Warning events log at Warn,
// Normal events at Info. Use it for milestone/state-change events whose reason
// codes should be visible (and alertable) in the default INFO log stream.
func Eventf(ctx context.Context, recorder events.EventRecorder, obj runtime.Object, eventtype string, reason errx.Reason, action, format string, args ...any) {
	eventf(ctx, Info, recorder, obj, eventtype, reason, action, format, args...)
}

// EventfDebug is Eventf for high-frequency, per-reconcile Normal events (e.g. a
// skipped optional sidecar) that would flood INFO. The paired log line drops to
// Debug while the K8s Event is still recorded for `kubectl describe`. Warning
// events still log at Warn.
func EventfDebug(ctx context.Context, recorder events.EventRecorder, obj runtime.Object, eventtype string, reason errx.Reason, action, format string, args ...any) {
	eventf(ctx, Debug, recorder, obj, eventtype, reason, action, format, args...)
}

func eventf(ctx context.Context, normalOpen func(context.Context, errx.Reason) *zerolog.Event, recorder events.EventRecorder, obj runtime.Object, eventtype string, reason errx.Reason, action, format string, args ...any) {
	open := normalOpen
	if eventtype == corev1.EventTypeWarning {
		open = Warn
	}
	open(ctx, reason).Msg(fmt.Sprintf(format, args...))

	if recorder != nil {
		recorder.Eventf(obj, nil, eventtype, reason.String(), action, format, args...)
	}
}

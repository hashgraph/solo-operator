// SPDX-License-Identifier: Apache-2.0

package common

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"

	"github.com/hashgraph/solo-operator/internal/errx"
)

// Eventf records a K8s event and a paired logx line with the same reason=
// (HIP-1496). Warning events log at Warn, others at Info; recorder may be nil.
func Eventf(ctx context.Context, recorder events.EventRecorder, obj runtime.Object, eventtype string, reason errx.Reason, action, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)

	open := Info
	if eventtype == corev1.EventTypeWarning {
		open = Warn
	}
	open(ctx, reason).Msg(msg)

	if recorder != nil {
		recorder.Eventf(obj, nil, eventtype, reason.String(), action, "%s", msg)
	}
}

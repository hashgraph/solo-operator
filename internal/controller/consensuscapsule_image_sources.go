// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math/rand/v2"
	"strings"

	"github.com/automa-saga/errx"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
	"github.com/hashgraph/solo-operator/internal/registry"
)

// sourcedContainer pairs a container that declares a SoftwareVersionSource with the
// address of its SoftwareVersion field, so a resolved, pinned image can be written back
// in place (a source-only container starts with a nil SoftwareVersion).
type sourcedContainer struct {
	name   string
	source *operatorv1alpha1.SoftwareVersionSource
	target **operatorv1alpha1.SoftwareVersion
}

// sourcedContainers returns every running container that declares a SoftwareVersionSource,
// mirroring configuredCapsuleImages' container set (consensus node always; enabled
// uploaders/UC/Alloy). Containers without a source are omitted.
func sourcedContainers(capsule *operatorv1alpha1.ConsensusCapsule) []sourcedContainer {
	ct := &capsule.Spec.PodProperties.Containers
	var out []sourcedContainer
	add := func(name string, src *operatorv1alpha1.SoftwareVersionSource, tgt **operatorv1alpha1.SoftwareVersion) {
		if src != nil {
			out = append(out, sourcedContainer{name: name, source: src, target: tgt})
		}
	}
	add("consensusNode", ct.ConsensusNode.SoftwareVersionSource, &ct.ConsensusNode.SoftwareVersion)
	if u := ct.RecordsUploader; u != nil && u.Enabled {
		add("recordsUploader", u.SoftwareVersionSource, &u.SoftwareVersion)
	}
	if u := ct.EventsUploader; u != nil && u.Enabled {
		add("eventsUploader", u.SoftwareVersionSource, &u.SoftwareVersion)
	}
	if u := ct.BlocksUploader; u != nil && u.Enabled {
		add("blocksUploader", u.SoftwareVersionSource, &u.SoftwareVersion)
	}
	if u := ct.BackupUploader; u != nil && u.Enabled {
		add("backupUploader", u.SoftwareVersionSource, &u.SoftwareVersion)
	}
	if uc := ct.UC; uc != nil && uc.Enabled {
		add("uc", uc.SoftwareVersionSource, &uc.SoftwareVersion)
	}
	if a := ct.Alloy; a != nil && a.Enabled {
		add("alloy", a.SoftwareVersionSource, &a.SoftwareVersion)
	}
	return out
}

// mismatchedSourceTag checks that every candidate registry in a source lists the
// same image tag. A deterministic source publishes one version to every registry,
// so the tags must match. It returns the first tag that differs from the first
// candidate's and mismatch=true, or mismatch=false when they all match. An empty
// tag is a legitimate value, not a sentinel, so a candidate with an empty tag that
// differs from the first is still reported as a mismatch (the returned string is
// then ""); the "at least one registry" check is done separately.
func mismatchedSourceTag(src *operatorv1alpha1.SoftwareVersionSource) (string, bool) {
	if src == nil || len(src.ImageRepositories) == 0 {
		return "", false
	}
	want := src.ImageRepositories[0].ImageTag
	for i := 1; i < len(src.ImageRepositories); i++ {
		if got := src.ImageRepositories[i].ImageTag; got != want {
			return got, true
		}
	}
	return "", false
}

// resolveImageSources resolves each container's SoftwareVersionSource to a verified,
// digest-pinned SoftwareVersion, writing it back into the spec in memory so the resource
// builders deploy the pinned image unchanged. It returns a non-nil *ctrl.Result when the
// caller must stop the reconcile (terminal config error, transient registry failure, or a
// status-write conflict); nil means "continue". Selections are cached in
// status.resolvedImages and reused while the source is unchanged.
func (r *ConsensusCapsuleReconciler) resolveImageSources(ctx context.Context, capsule *operatorv1alpha1.ConsensusCapsule) (*ctrl.Result, error) {
	sourced := sourcedContainers(capsule)

	// Drop cached entries for containers that no longer declare a source.
	statusChanged := pruneResolvedImages(capsule, sourced)

	// Resolved pins are collected here and applied only AFTER the status write below:
	// client.Status().Update reloads the server's spec into the object, which would
	// otherwise clobber an in-memory SoftwareVersion pin written before it.
	type pendingPin struct {
		target   **operatorv1alpha1.SoftwareVersion
		resolved *operatorv1alpha1.SoftwareVersion
	}
	var pins []pendingPin

	var checker registry.ImageChecker
	rng := rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
	requeue := false

	for _, sc := range sourced {
		comp := sc.source.ToImageComponentSpec()

		// Fail-closed: a source must have candidate registries and usable per-platform
		// layer hashes. (CRD validation enforces this too; this guards older CRDs and
		// programmatic clients.)
		if len(comp.Registries) == 0 || len(comp.Deterministic.LayerHashes) == 0 {
			return r.failImageSourceInvalid(ctx, capsule, errx.WithHints(
				errx.WithReason(errImageSourceInvalid.New(
					"container %q softwareVersionSource is invalid: needs at least one imageRepository and per-platform layer hashes (os + architecture)", sc.name),
					ReasonImageSourceInvalid),
				"Set imageRepositories and imageVerificationSpec (os, architecture, layerHashes) on the source, or use a single softwareVersion instead.",
			))
		}

		// A source must list the same version in every candidate registry. The shared
		// layer hashes verify the image content, but a mismatched tag means the
		// candidates are not the same version, so registry selection could deploy a
		// different version depending on which registry it picks. Fail closed on that.
		//
		// This runs in Go, not as a CRD rule: a CRD rule that loops over the registry
		// list would exceed the API server's cost limit once the list is large.
		//
		// softwareVersion and softwareVersionSource are mutually exclusive (the CRD's
		// exactly-one / at-most-one rule enforces that), so there is no cross-field tag
		// to check here — only that the source is internally consistent.
		if bad, mismatch := mismatchedSourceTag(sc.source); mismatch {
			return r.failImageSourceInvalid(ctx, capsule, errx.WithHints(
				errx.WithReason(errImageSourceInvalid.New(
					"container %q softwareVersionSource lists differing image tags across its candidate registries (e.g. %q); a deterministic source must publish the same version in every registry", sc.name, bad),
					ReasonImageSourceInvalid),
				"Make every imageRepositories[*].imageTag identical (the same version in each registry).",
			))
		}

		hash, err := sourceHash(sc.source)
		if err != nil {
			return &ctrl.Result{}, err
		}

		// Cache hit: reuse the pinned image, skip registry inspection. This is steady
		// state (the source is unchanged), so it is logged at debug rather than emitting
		// an event — the selection is already durable in status.resolvedImages and the
		// RegistrySelected event fired when it was first chosen.
		if cached := findResolvedImage(capsule, sc.name); cached != nil &&
			cached.SourceHash == hash &&
			cached.Validity == operatorv1alpha1.ImageValidityValid &&
			cached.Digest != "" {
			if resolved, ok := softwareVersionFromCache(sc.source, cached); ok {
				pins = append(pins, pendingPin{target: sc.target, resolved: resolved})
				common.Debug(ctx, ReasonImageSourceResolved).
					Str("container", sc.name).Str("image", cached.Image).Msg("Reusing cached resolved image")
				continue
			}
			// Cache reference no longer maps to a candidate — fall through and re-resolve.
		}

		// Build the checker lazily (once per reconcile) with a keychain derived from the
		// capsule's image-pull secrets so private candidates can be inspected.
		if checker == nil {
			c, err := r.imageChecker(ctx, capsule, sourced)
			if err != nil {
				return &ctrl.Result{}, err
			}
			checker = c
		}

		_, winner, ok := registry.SelectComponent(ctx, checker, comp, checkTimeout, rng, r.StrategyName)
		if !ok {
			setResolvedImage(capsule, operatorv1alpha1.ResolvedImageStatus{
				Container:  sc.name,
				SourceHash: hash,
				Validity:   operatorv1alpha1.ImageValidityInvalid,
			})
			statusChanged = true
			requeue = true
			r.eventImageSource(capsule, corev1.EventTypeWarning, operatorv1alpha1.EventRegistryInvalid,
				"%s: no valid registry available for softwareVersionSource", sc.name)
			common.Warn(ctx, ReasonNoValidRegistry).Str("container", sc.name).Msg("Found no valid registry for softwareVersionSource")
			continue
		}

		repo, name, tag := common.ParseImage(winner.Image)
		resolved := &operatorv1alpha1.SoftwareVersion{
			Repository:       repo,
			ImageName:        name,
			ImageTag:         tag,
			Digest:           winner.Digest,
			ImagePullPolicy:  sc.source.ImagePullPolicy,
			ImagePullSecrets: pullSecretsForRef(sc.source, winner.Image),
		}
		pins = append(pins, pendingPin{target: sc.target, resolved: resolved})
		setResolvedImage(capsule, operatorv1alpha1.ResolvedImageStatus{
			Container:           sc.name,
			SourceHash:          hash,
			Image:               common.BuildImage(resolved),
			Digest:              winner.Digest,
			Repository:          repo,
			LayerHashesVerified: winner.LayerHashesVerified,
			Validity:            operatorv1alpha1.ImageValidityValid,
		})
		statusChanged = true
		r.eventImageSource(capsule, corev1.EventTypeNormal, operatorv1alpha1.EventRegistrySelected,
			"%s: selected %s (digest %s, layerHashesVerified %v)", sc.name, winner.Image, winner.Digest, winner.LayerHashesVerified)
		common.Info(ctx, ReasonImageSourceResolved).Str("container", sc.name).Str("image", winner.Image).Msg("Resolved softwareVersionSource")
	}

	if statusChanged {
		if res, err := r.persistResolvedImages(ctx, capsule); res != nil {
			return res, err
		}
	}

	// Apply the resolved pins now — after the status write above, which reloaded the
	// server's (unpinned) spec into the object. The subreconciler build is the next
	// consumer of these fields and no capsule status write happens in between.
	for _, p := range pins {
		*p.target = p.resolved
	}

	if requeue {
		return &ctrl.Result{RequeueAfter: podReadinessRequeueAfter}, nil
	}
	return nil, nil
}

// imageChecker returns the injected test checker, or builds an OCIChecker whose keychain
// is derived from the union of the sources' image-pull secrets.
func (r *ConsensusCapsuleReconciler) imageChecker(ctx context.Context, capsule *operatorv1alpha1.ConsensusCapsule, sourced []sourcedContainer) (registry.ImageChecker, error) {
	if r.Checker != nil {
		return r.Checker, nil
	}
	kc, err := registry.KeychainFromPullSecrets(ctx, r.Client, capsule.Namespace, sourcePullSecretRefs(sourced))
	if err != nil {
		return nil, err
	}
	return registry.OCIChecker{Keychain: kc, AllowedHosts: r.AllowedRegistryHosts}, nil
}

// sourcePullSecretRefs is the deduplicated union of every candidate repository's
// ImagePullSecrets across the sourced containers.
func sourcePullSecretRefs(sourced []sourcedContainer) []corev1.LocalObjectReference {
	var groups [][]corev1.LocalObjectReference
	for _, sc := range sourced {
		for i := range sc.source.ImageRepositories {
			groups = append(groups, sc.source.ImageRepositories[i].ImagePullSecrets)
		}
	}
	return common.MergeImagePullSecrets(groups...)
}

// pullSecretsForRef returns the ImagePullSecrets of the candidate repository whose
// tag-only reference matches ref (the selected winner's image), or nil.
func pullSecretsForRef(src *operatorv1alpha1.SoftwareVersionSource, ref string) []corev1.LocalObjectReference {
	if repo := repoByRef(src, ref); repo != nil {
		return repo.ImagePullSecrets
	}
	return nil
}

// repoByRef finds the candidate repository whose tag-only reference equals ref.
func repoByRef(src *operatorv1alpha1.SoftwareVersionSource, ref string) *operatorv1alpha1.ImageRepository {
	for i := range src.ImageRepositories {
		if src.ImageRepositories[i].Ref() == ref {
			return &src.ImageRepositories[i]
		}
	}
	return nil
}

// softwareVersionFromCache reconstructs a pinned SoftwareVersion from a cached selection,
// mapping the cached reference back to its candidate repository so pull secrets and pull
// policy come from the source and the digest comes from the cache. It returns ok=false when
// the cached reference no longer maps to any candidate — the caller must then treat it as a
// cache miss and re-resolve, rather than deploy a reference stripped of its pull secrets
// (which would fail to pull from a private registry).
func softwareVersionFromCache(src *operatorv1alpha1.SoftwareVersionSource, cached *operatorv1alpha1.ResolvedImageStatus) (*operatorv1alpha1.SoftwareVersion, bool) {
	ref := strings.SplitN(cached.Image, "@", 2)[0]
	repo := repoByRef(src, ref)
	if repo == nil {
		return nil, false
	}
	return &operatorv1alpha1.SoftwareVersion{
		Repository:       repo.Repository,
		ImageName:        repo.ImageName,
		ImageTag:         repo.ImageTag,
		Digest:           cached.Digest,
		ImagePullPolicy:  src.ImagePullPolicy,
		ImagePullSecrets: repo.ImagePullSecrets,
	}, true
}

// sourceHash fingerprints a SoftwareVersionSource so a spec change invalidates the cache.
// It returns an error rather than an empty key on marshal failure: an empty key would
// silently disable the cache and re-probe every registry on every reconcile.
func sourceHash(src *operatorv1alpha1.SoftwareVersionSource) (string, error) {
	b, err := json.Marshal(src)
	if err != nil {
		return "", errx.WithHints(
			errx.WithReason(errReconcile.Wrap(err, "marshalling softwareVersionSource for cache key"), ReasonSourceHashFailed),
			"Confirm the container's softwareVersionSource is well-formed",
		)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func findResolvedImage(capsule *operatorv1alpha1.ConsensusCapsule, container string) *operatorv1alpha1.ResolvedImageStatus {
	for i := range capsule.Status.ResolvedImages {
		if capsule.Status.ResolvedImages[i].Container == container {
			return &capsule.Status.ResolvedImages[i]
		}
	}
	return nil
}

// setResolvedImage upserts a resolved-image status entry by container name, stamping the
// probe time so callers don't have to.
func setResolvedImage(capsule *operatorv1alpha1.ConsensusCapsule, entry operatorv1alpha1.ResolvedImageStatus) {
	entry.LastProbeTime = metav1.Now()
	if existing := findResolvedImage(capsule, entry.Container); existing != nil {
		*existing = entry
		return
	}
	capsule.Status.ResolvedImages = append(capsule.Status.ResolvedImages, entry)
}

// pruneResolvedImages removes status entries for containers no longer using a source and
// reports whether anything was removed.
func pruneResolvedImages(capsule *operatorv1alpha1.ConsensusCapsule, sourced []sourcedContainer) bool {
	if len(capsule.Status.ResolvedImages) == 0 {
		return false
	}
	keep := make(map[string]bool, len(sourced))
	for _, sc := range sourced {
		keep[sc.name] = true
	}
	kept := make([]operatorv1alpha1.ResolvedImageStatus, 0, len(capsule.Status.ResolvedImages))
	changed := false
	for _, entry := range capsule.Status.ResolvedImages {
		if keep[entry.Container] {
			kept = append(kept, entry)
		} else {
			changed = true
		}
	}
	capsule.Status.ResolvedImages = kept
	return changed
}

// persistResolvedImages writes status.resolvedImages. It returns a requeue result on a
// conflict, a wrapped error on other failures, and nil to continue.
func (r *ConsensusCapsuleReconciler) persistResolvedImages(ctx context.Context, capsule *operatorv1alpha1.ConsensusCapsule) (*ctrl.Result, error) {
	if err := r.Status().Update(ctx, capsule); err != nil {
		if apierrors.IsConflict(err) {
			return &ctrl.Result{Requeue: true}, nil
		}
		return &ctrl.Result{}, wrapStatusUpdate(err, "updating resolvedImages status")
	}
	return nil, nil
}

// failImageSourceInvalid records a terminal invalid-source config error (condition +
// warning event + Failed phase) and stops the reconcile without a requeue. Mirrors
// failImageRequired.
func (r *ConsensusCapsuleReconciler) failImageSourceInvalid(ctx context.Context, capsule *operatorv1alpha1.ConsensusCapsule, err error) (*ctrl.Result, error) {
	log.FromContext(ctx).Error(err, "softwareVersionSource is invalid", "reason", ReasonImageSourceInvalid.String())
	if r.Recorder != nil {
		r.Recorder.Eventf(capsule, nil, corev1.EventTypeWarning, operatorv1alpha1.EventImageSourceInvalid.String(),
			"Reconcile", "%s", errx.Format(err))
	}
	capsule.Status.Phase = operatorv1alpha1.PhaseFailed
	capsule.Status.ObservedGeneration = capsule.Generation
	apimeta.SetStatusCondition(&capsule.Status.Conditions, metav1.Condition{
		Type:               operatorv1alpha1.CondImagesDeclared,
		Status:             metav1.ConditionFalse,
		ObservedGeneration: capsule.Generation,
		Reason:             ReasonImageSourceInvalid.String(),
		Message:            err.Error(),
	})
	if uerr := r.Status().Update(ctx, capsule); uerr != nil {
		if apierrors.IsConflict(uerr) {
			return &ctrl.Result{Requeue: true}, nil
		}
		return &ctrl.Result{}, uerr
	}
	return &ctrl.Result{}, nil
}

// eventImageSource emits an image-source selection event on the capsule.
func (r *ConsensusCapsuleReconciler) eventImageSource(capsule *operatorv1alpha1.ConsensusCapsule, eventtype string, reason errx.Reason, format string, args ...any) {
	if r.Recorder != nil {
		r.Recorder.Eventf(capsule, nil, eventtype, reason.String(), "ImageSource", format, args...)
	}
}

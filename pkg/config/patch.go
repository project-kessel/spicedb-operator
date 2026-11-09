package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	jsonpatch "github.com/evanphx/json-patch"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	applymetav1 "k8s.io/client-go/applyconfigurations/meta/v1"

	"github.com/authzed/spicedb-operator/pkg/apis/authzed/v1alpha1"
)

const wildcard = "*"

// protectedRBACFields maps generated RBAC kinds to the top-level field whose
// modification by a user-supplied patch would let that patch escalate the
// permissions the operator grants to SpiceDB.
var protectedRBACFields = map[string]string{
	"Role":        "rules",
	"RoleBinding": "roleRef",
}

// validateRBACPatch rejects patches that modify the security-sensitive field
// of a generated Role or RoleBinding. ApplyPatches matches patches by kind
// only, so without this check a SpiceDBCluster author could rewrite the
// operator-managed rules or roleRef and escalate the ServiceAccount's
// privileges beyond what the operator grants.
func validateRBACPatch(kind string, patch json.RawMessage) error {
	field, protected := protectedRBACFields[kind]
	if !protected {
		return nil
	}

	// Determine whether the patch is a JSON6902 operation or a strategic
	// merge patch, mirroring the detection in ApplyPatches.
	decoder := utilyaml.NewYAMLOrJSONDecoder(bytes.NewReader(patch), 100)
	var op jsonpatch.Operation
	if err := decoder.Decode(&op); err == nil && op.Kind() != "unknown" {
		// test operations do not modify the object.
		if op.Kind() == "test" {
			return nil
		}
		path, err := op.Path()
		if err != nil {
			return nil
		}
		if path == "" || path == "/" || path == "/"+field || strings.HasPrefix(path, "/"+field+"/") {
			return fmt.Errorf("patch may not modify %q on a %s", field, kind)
		}
		return nil
	}

	// Strategic merge patch: the protected fields are both top-level, so only
	// the top-level keys matter here.
	jsonPatch, err := utilyaml.ToJSON(patch)
	if err != nil {
		return nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(jsonPatch, &fields); err != nil {
		return nil
	}
	if _, ok := fields[field]; ok {
		return fmt.Errorf("patch may not modify %q on a %s", field, kind)
	}
	return nil
}

// ApplyPatches applies a set of patches to an object.
// It returns the number of patches applied, a bool indicating whether there
// were matching patches and the input differed from the output, and any errors
// that occurred.
//
// resolver is only consulted for strategic merge patches, which need the
// apiserver's schema to determine merge keys. json6902 patches and clusters
// with no patches never touch it, so it is passed as a resolver and consulted
// on demand rather than resolved eagerly at operator startup.
func ApplyPatches[K any](object, out K, patches []v1alpha1.Patch, resolver PatchMetaResolver) (int, bool, error) {
	// marshal object to json for patching
	encoded, err := json.Marshal(object)
	if err != nil {
		return 0, false, fmt.Errorf("error marshalling object to patch: %w", err)
	}

	initial := encoded

	// HACK: Unmarshal into TypeMeta to determine `kind` of incoming object.
	// The ApplyConfiguration objects don't have any getters, so there's no
	// common interface to use to get their `kind`, even though they all have
	// a kind field. Golang also doesn't support writing generic functions over
	// struct members. This hack can be removed if we add getters to the
	// generated applyconfigurations or golang supports this via generics:
	// - https://github.com/golang/go/issues/51259
	// - https://github.com/golang/go/issues/48522
	// - https://github.com/kubernetes/kubernetes/issues/113773
	var typeMeta applymetav1.TypeMetaApplyConfiguration
	if err := json.Unmarshal(encoded, &typeMeta); err != nil {
		return 0, false, fmt.Errorf("unable to extract type info from object for patching: %w", err)
	}

	if typeMeta.Kind == nil {
		return 0, false, fmt.Errorf("object doesn't specify kind: %v", object)
	}

	count := 0
	errs := make([]error, 0)
	for i, p := range patches {
		if p.Kind == *typeMeta.Kind || p.Kind == wildcard {
			if err := validateRBACPatch(*typeMeta.Kind, p.Patch); err != nil {
				errs = append(errs, fmt.Errorf("patch %d rejected: %w", i, err))
				continue
			}

			// determine if the patch is a strategic merge or a json6902 patch
			decoder := utilyaml.NewYAMLOrJSONDecoder(bytes.NewReader(p.Patch), 100)
			var json6902op jsonpatch.Operation
			err = decoder.Decode(&json6902op)
			if err != nil {
				errs = append(errs, fmt.Errorf("error decoding patch %d: %w", i, err))
				continue
			}

			// if there's no operation, it's a strategic merge patch, not 6902
			if json6902op.Kind() == "unknown" {
				jsonPatch, err := utilyaml.ToJSON(p.Patch)
				if err != nil {
					errs = append(errs, fmt.Errorf("error converting patch %d to json: %w", i, err))
					continue
				}
				gv, err := schema.ParseGroupVersion(*typeMeta.APIVersion)
				if err != nil {
					errs = append(errs, fmt.Errorf("error applying patch %d, to object: %w", i, err))
					continue
				}
				// Resolved here, at the only point that needs it, and with the
				// GVK in hand -- which is what lets the v3 resolver fetch a
				// single group-version instead of the whole cluster's schema.
				lookupPatchMeta, err := resolver.LookupPatchMeta(gv.WithKind(*typeMeta.Kind))
				if err != nil {
					errs = append(errs, fmt.Errorf("error applying patch %d, couldn't resolve patch metadata: %w", i, err))
					continue
				}
				patched, err := strategicpatch.StrategicMergePatchUsingLookupPatchMeta(encoded, jsonPatch, lookupPatchMeta)
				if err != nil {
					errs = append(errs, fmt.Errorf("error applying patch %d, to object: %w", i, err))
					continue
				}
				encoded = patched
			} else {
				json6902patch := jsonpatch.Patch([]jsonpatch.Operation{json6902op})
				patched, err := json6902patch.Apply(encoded)
				if err != nil {
					errs = append(errs, fmt.Errorf("error applying patch %d to object: %w", i, err))
					continue
				}
				encoded = patched
			}
			count++
		}
	}

	if err := json.Unmarshal(encoded, out); err != nil {
		errs = append(errs, fmt.Errorf("error converting back to object: %w", err))
	}

	diff := false
	if count > 0 {
		// return true if there were patches defined and the output bytes differ
		diff = !bytes.Equal(encoded, initial)
	}
	return count, diff, kerrors.NewAggregate(errs)
}

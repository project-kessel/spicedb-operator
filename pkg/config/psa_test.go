package config

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
)

var wantNamespacePSALabels = map[string]string{
	"pod-security.kubernetes.io/enforce":         "restricted",
	"pod-security.kubernetes.io/enforce-version": "latest",
	"pod-security.kubernetes.io/warn":            "restricted",
	"pod-security.kubernetes.io/warn-version":    "latest",
	"pod-security.kubernetes.io/audit":           "restricted",
	"pod-security.kubernetes.io/audit-version":   "latest",
}

func TestOperatorNamespacePSALabels(t *testing.T) {
	manifests := []string{
		"../../config/operator.yaml",
		"../../config/operator_openshift.yaml",
		"../../deploy/deploy.yml",
	}

	for _, manifest := range manifests {
		t.Run(filepath.Base(manifest), func(t *testing.T) {
			namespaces := namespacesFromManifest(t, manifest)
			require.NotEmpty(t, namespaces, "expected a Namespace in %s", manifest)

			for _, namespace := range namespaces {
				metadata, ok := namespace["metadata"].(map[string]any)
				require.True(t, ok, "Namespace in %s has no metadata", manifest)
				labels, ok := metadata["labels"].(map[string]any)
				require.True(t, ok, "Namespace in %s has no labels", manifest)

				for key, want := range wantNamespacePSALabels {
					require.Equal(t, want, labels[key], "%s Namespace label %s", manifest, key)
				}
			}
		})
	}
}

// namespacesFromManifest returns every Namespace document in a YAML file,
// including ones nested under an OpenShift Template's objects list.
func namespacesFromManifest(t *testing.T, path string) []map[string]any {
	t.Helper()

	file, err := os.Open(path)
	require.NoError(t, err)
	defer file.Close()

	decoder := utilyaml.NewYAMLOrJSONDecoder(file, 100)
	namespaces := make([]map[string]any, 0, 1)
	for {
		var doc map[string]any
		err := decoder.Decode(&doc)
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err)
		if doc == nil {
			continue
		}

		if doc["kind"] == "Namespace" {
			namespaces = append(namespaces, doc)
		}
		if objects, ok := doc["objects"].([]any); ok {
			for _, object := range objects {
				obj, ok := object.(map[string]any)
				if ok && obj["kind"] == "Namespace" {
					namespaces = append(namespaces, obj)
				}
			}
		}
	}
	return namespaces
}

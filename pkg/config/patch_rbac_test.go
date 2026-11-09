package config

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	applyrbacv1 "k8s.io/client-go/applyconfigurations/rbac/v1"
	"k8s.io/utils/ptr"

	"github.com/authzed/spicedb-operator/pkg/apis/authzed/v1alpha1"
)

func TestApplyPatchesRejectsRBACEscalation(t *testing.T) {
	resources := newTestPatchMetaResolver()
	tests := []struct {
		name        string
		kind        string
		patchKind   string
		patch       string
		wantErr     string
		wantApplied bool
	}{
		{
			name:      "role strategic merge rules",
			kind:      "Role",
			patchKind: "Role",
			patch: `rules:
- apiGroups: [""]
  resources: ["secrets"]
  verbs: ["get"]`,
			wantErr: `patch may not modify "rules" on a Role`,
		},
		{
			name:      "role json6902 add rules",
			kind:      "Role",
			patchKind: "Role",
			patch: `op: add
path: /rules/-
value:
  apiGroups: [""]
  resources: ["secrets"]
  verbs: ["get"]`,
			wantErr: `patch may not modify "rules" on a Role`,
		},
		{
			name:      "role json6902 edit existing rule",
			kind:      "Role",
			patchKind: "Role",
			patch:     `{"op": "replace", "path": "/rules/0/verbs", "value": ["*"]}`,
			wantErr:   `patch may not modify "rules" on a Role`,
		},
		{
			name:      "role json6902 replace whole object",
			kind:      "Role",
			patchKind: "Role",
			patch:     `{"op": "replace", "path": "/", "value": {}}`,
			wantErr:   `patch may not modify "rules" on a Role`,
		},
		{
			name:        "role strategic merge labels allowed",
			kind:        "Role",
			patchKind:   "Role",
			patch:       "metadata:\n  labels:\n    added: via-patch",
			wantApplied: true,
		},
		{
			name:        "role json6902 labels allowed",
			kind:        "Role",
			patchKind:   "Role",
			patch:       `{"op": "add", "path": "/metadata/labels", "value": {"added": "via-patch"}}`,
			wantApplied: true,
		},
		{
			name:      "rolebinding strategic merge roleRef",
			kind:      "RoleBinding",
			patchKind: "RoleBinding",
			patch: `roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin`,
			wantErr: `patch may not modify "roleRef" on a RoleBinding`,
		},
		{
			name:      "rolebinding json6902 replace roleRef name",
			kind:      "RoleBinding",
			patchKind: "RoleBinding",
			patch:     `{"op": "replace", "path": "/roleRef/name", "value": "cluster-admin"}`,
			wantErr:   `patch may not modify "roleRef" on a RoleBinding`,
		},
		{
			name:      "rolebinding json6902 add roleRef",
			kind:      "RoleBinding",
			patchKind: "RoleBinding",
			patch:     `{"op": "add", "path": "/roleRef", "value": {"kind": "ClusterRole", "name": "cluster-admin"}}`,
			wantErr:   `patch may not modify "roleRef" on a RoleBinding`,
		},
		{
			name:        "rolebinding strategic merge labels allowed",
			kind:        "RoleBinding",
			patchKind:   "RoleBinding",
			patch:       "metadata:\n  labels:\n    added: via-patch",
			wantApplied: true,
		},
		{
			name:      "wildcard patch cannot change role rules",
			kind:      "Role",
			patchKind: wildcard,
			patch: `rules:
- apiGroups: [""]
  resources: ["secrets"]
  verbs: ["get"]`,
			wantErr: `patch may not modify "rules" on a Role`,
		},
		{
			name:      "wildcard patch cannot change rolebinding roleRef",
			kind:      "RoleBinding",
			patchKind: wildcard,
			patch:     `{"op": "replace", "path": "/roleRef/name", "value": "cluster-admin"}`,
			wantErr:   `patch may not modify "roleRef" on a RoleBinding`,
		},
		{
			name:        "wildcard metadata patch still allowed",
			kind:        "Role",
			patchKind:   wildcard,
			patch:       `{"op": "add", "path": "/metadata/labels", "value": {"added": "via-patch"}}`,
			wantApplied: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var object any
			switch tt.kind {
			case "Role":
				object = applyrbacv1.Role("test", "test")
			case "RoleBinding":
				object = applyrbacv1.RoleBinding("test", "test")
			default:
				t.Fatalf("unsupported kind %q", tt.kind)
			}

			count, applied, err := ApplyPatches[any](object, object, []v1alpha1.Patch{{
				Kind:  tt.patchKind,
				Patch: json.RawMessage(tt.patch),
			}}, resources)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				require.Zero(t, count)
				require.False(t, applied)
				return
			}
			require.NoError(t, err)
			require.Equal(t, 1, count)
			require.Equal(t, tt.wantApplied, applied)
		})
	}
}

func TestNewConfigRejectsRBACEscalation(t *testing.T) {
	resources := newTestPatchMetaResolver()
	tests := []struct {
		name      string
		patchKind string
		patch     string
	}{
		{
			name:      "role rules",
			patchKind: "Role",
			patch: `rules:
- apiGroups: [""]
  resources: ["secrets"]
  verbs: ["get"]`,
		},
		{
			name:      "rolebinding roleRef",
			patchKind: "RoleBinding",
			patch: `roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin`,
		},
		{
			name:      "wildcard rules",
			patchKind: wildcard,
			patch: `rules:
- apiGroups: [""]
  resources: ["secrets"]
  verbs: ["get"]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			secret := &corev1.Secret{Data: map[string][]byte{
				"datastore_uri": []byte("uri"),
				"preshared_key": []byte("psk"),
			}}
			cluster := &v1alpha1.SpiceDBCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
					UID:       types.UID("1"),
				},
				Spec: v1alpha1.ClusterSpec{
					SecretRef: "test-secret",
					Config: json.RawMessage(`
						{
							"logLevel": "debug",
							"datastoreEngine": "cockroachdb"
						}
					`),
					Patches: []v1alpha1.Patch{{
						Kind:  tt.patchKind,
						Patch: json.RawMessage(tt.patch),
					}},
				},
			}

			_, _, err := NewConfig(cluster, ptr.To(testGlobalConfig.Copy()), singleSecretMap("test-secret", secret), resources)
			require.Error(t, err)
			require.Contains(t, err.Error(), "rejected")
		})
	}
}

package origin

import "testing"

func TestClassify(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"github.com/openshift/api/hypershift/v1beta1", "openshift/api"},
		{"github.com/openshift/library-go/pkg/operator", "openshift/library-go"},
		{"sigs.k8s.io/cluster-api/api/v1beta1", "kubernetes-sigs/cluster-api"},
		{"k8s.io/api/core/v1", "kubernetes/api"},
		{"k8s.io/apimachinery/pkg/apis/meta/v1", "kubernetes/apimachinery"},
		{"k8s.io/client-go/kubernetes", "kubernetes/client-go"},
		{"sigs.k8s.io/controller-runtime/pkg/client", "kubernetes-sigs/controller-runtime"},
		{"github.com/openshift/hypershift/support/config", "openshift/hypershift"},
		{"github.com/some/unknown/pkg", ""},
		{"context", ""},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			got := Classify(tc.path)
			if got != tc.want {
				t.Errorf("Classify(%q) = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}

func TestIsStdLib(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"context", true},
		{"fmt", true},
		{"os/exec", true},
		{"net/http", true},
		{"github.com/openshift/api", false},
		{"k8s.io/api", false},
		{"sigs.k8s.io/controller-runtime", false},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			got := IsStdLib(tc.path)
			if got != tc.want {
				t.Errorf("IsStdLib(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

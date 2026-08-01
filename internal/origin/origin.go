package origin

import "strings"

var known = map[string]string{
	"github.com/openshift/hypershift":                    "openshift/hypershift",
	"github.com/openshift/api":                           "openshift/api",
	"github.com/openshift/library-go":                    "openshift/library-go",
	"sigs.k8s.io/cluster-api":                            "kubernetes-sigs/cluster-api",
	"k8s.io/api":                                         "kubernetes/api",
	"k8s.io/apimachinery":                                "kubernetes/apimachinery",
	"k8s.io/client-go":                                   "kubernetes/client-go",
	"k8s.io/utils":                                       "kubernetes/utils",
	"sigs.k8s.io/controller-runtime":                     "kubernetes-sigs/controller-runtime",
	"github.com/prometheus-operator/prometheus-operator":  "prometheus-operator/prometheus-operator",
	"github.com/openshift/cluster-api-provider-agent":     "openshift/cluster-api-provider-agent",
	"github.com/openshift/cluster-api-provider-openstack": "openshift/cluster-api-provider-openstack",
}

// Classify returns the repository name for an import path by matching
// against known prefixes. Returns "" if no match.
func Classify(importPath string) string {
	best := ""
	bestLen := 0
	for prefix, repo := range known {
		if strings.HasPrefix(importPath, prefix) && len(prefix) > bestLen {
			best = repo
			bestLen = len(prefix)
		}
	}
	return best
}

// IsStdLib returns true if the import path is a Go standard library package.
func IsStdLib(importPath string) bool {
	first := importPath
	if i := strings.Index(importPath, "/"); i >= 0 {
		first = importPath[:i]
	}
	return !strings.Contains(first, ".")
}

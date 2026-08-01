package testdata

import "fmt"

func BuildCertSANs(namespace string) []string {
	return []string{
		fmt.Sprintf("*.etcd-discovery.%s.svc", namespace),
		fmt.Sprintf("*.etcd-discovery.%s.svc.cluster.local", namespace),
		"127.0.0.1",
		"::1",
		"ok",
	}
}

func SimpleFunc() {
	x := "no-structural-chars"
	_ = x
}

package testdata

import (
	"fmt"

	certificatesv1alpha1 "example.com/api/v1alpha1"
)

func setCondition() {
	_ = certificatesv1alpha1.PreviousCertificatesRevokedType
	_ = certificatesv1alpha1.NewCertificatesTrustedType
	_ = fmt.Sprintf("short.path/%s", "x")
	shortPkg.Do()
}

package envtest

import "os"

func configureFromEnv() {
	region := os.Getenv("AWS_REGION")
	_ = region
	platforms := os.Getenv("PLATFORMS_INSTALLED")
	_ = platforms
}

func noEnvVars() {
	x := 1 + 2
	_ = x
}

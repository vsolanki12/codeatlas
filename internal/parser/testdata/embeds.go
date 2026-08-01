package testdata

import "embed"

//go:embed */*.yaml
var manifestAssets embed.FS

//go:embed init-script.sh
var initScript string

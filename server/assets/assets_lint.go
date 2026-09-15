//go:build server && sliver_lint

package assets

import (
	"embed"
	"io/fs"
)

// Linting does not execute the embedded toolchains, so it can use an empty
// filesystem instead of downloading the generated asset bundle.
var assetsFs embed.FS

func assetsFS() fs.FS { return assetsFs }

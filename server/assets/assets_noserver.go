//go:build !server

package assets

import (
	"embed"
	"io/fs"
)

// 非 server 构建（主 module 的 go build ./... / go vet / gopls）不打包 zig/garble
// 等生成资产（见 assets_<os>_<arch>.go 的 server 构建约束），用空文件系统占位，
// 保证整棵 sliver 源码树在无标签模式下可编译/可静态检查。实际 server 二进制始终
// 带 server 标签构建（scripts/build-sliver.sh），行为不受影响；与上游
// assets_lint.go 的空 FS 占位同思路。
var assetsFs embed.FS

func assetsFS() fs.FS { return assetsFs }

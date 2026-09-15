//go:build server && !sliver_lint && !sliver_assets_disk

package assets

import (
	"embed"
	"io/fs"
)

var (
	//go:embed fs/*.txt fs/*.zip fs/windows/amd64/*
	assetsFs embed.FS
)

// assetsFS 返回资产文件系统：默认构建为编译期内嵌的只读 FS
// （sliver_assets_disk 模式下改由 assets_disk.go 提供磁盘实现）。
func assetsFS() fs.FS { return assetsFs }

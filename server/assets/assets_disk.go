//go:build server && !sliver_lint && sliver_assets_disk

package assets

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// 磁盘资产模式（构建标签 sliver_assets_disk）：资产（go.zip / src.zip / zig /
// garble / 词表）不打进二进制，改为运行时从磁盘目录读取。
//
// 用途：平台把 sliver server 内嵌进主二进制时，避免 Go 工具链等约 140MB 资产
// 撑大主二进制、拖慢每次构建与备份；资产目录可随部署包分发或裁剪（仅纯 Go
// 植入体只需 src.zip + go.zip）。
//
// 目录解析优先级（延迟到首次读取，平台可在此之前设置环境变量）：
//  1. SLIVER_ASSETS_DIR 显式指定
//  2. <SLIVER_ROOT_DIR>/assets（即 root dir 下的 assets/ 子目录）
const assetsDirEnvVarName = "SLIVER_ASSETS_DIR"

var (
	assetsFSOnce sync.Once
	assetsFSVal  fs.FS
)

// assetsFS 返回磁盘资产文件系统（进程内只解析一次目录）。
func assetsFS() fs.FS {
	assetsFSOnce.Do(func() {
		dir := strings.TrimSpace(os.Getenv(assetsDirEnvVarName))
		if dir == "" {
			dir = filepath.Join(GetRootAppDir(), "assets")
		}
		assetsFSVal = os.DirFS(dir)
	})
	return assetsFSVal
}

package assets

import "io/fs"

// 资产数据源统一入口。具体来源由构建标签决定：
//   - 默认（无 sliver_assets_disk）：编译期内嵌 //go:embed，见 assets_<os>_<arch>.go
//   - sliver_assets_disk：运行时从磁盘目录读取，见 assets_disk.go
//
// 所有资产读取（fs/... 相对路径）都经过 readAsset，替换数据源时只需改 assetsFS()。
func readAsset(name string) ([]byte, error) {
	return fs.ReadFile(assetsFS(), name)
}

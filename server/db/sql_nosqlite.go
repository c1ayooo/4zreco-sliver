//go:build !cgo_sqlite && !go_sqlite && !wasm_sqlite

package db

import (
	"4zreco/var/sliver/server/configs"
	"gorm.io/gorm"
)

// 未选择 SQLite 驱动标签（go_sqlite / cgo_sqlite / wasm_sqlite）的构建占位：
// 仅用于让主 module 的 ./... 通配构建与静态检查通过；server 二进制必须带驱动
// 标签构建（scripts/build-sliver.sh 使用 go_sqlite）。此处被实际调用时会以明确
// 信息 panic，避免产出"能编译但静默不可用"的 server。
func sqliteClient(*configs.DatabaseConfig) *gorm.DB {
	panic("sliver server 需要 SQLite 驱动构建标签：go_sqlite（推荐）/ cgo_sqlite / wasm_sqlite")
}

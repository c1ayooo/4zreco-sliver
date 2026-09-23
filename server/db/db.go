package db

import (
	"sync"

	"gorm.io/gorm"
)

/*
	Sliver Implant Framework
	Copyright (C) 2019  Bishop Fox

	This program is free software: you can redistribute it and/or modify
	it under the terms of the GNU General Public License as published by
	the Free Software Foundation, either version 3 of the License, or
	(at your option) any later version.

	This program is distributed in the hope that it will be useful,
	but WITHOUT ANY WARRANTY; without even the implied warranty of
	MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
	GNU General Public License for more details.

	You should have received a copy of the GNU General Public License
	along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

// Client - Database Client
//
// 惰性初始化：首次 Session() 时才按 root dir 下 configs/database.yaml 建连。
// 内嵌 server（平台单进程）需要先设置 SLIVER_ROOT_DIR 并写好数据库配置再触发建连；
// 惰性化同时避免 import 期 panic（无 sqlite 驱动标签 + sqlite 方言时占位实现会 panic）。
var Client *gorm.DB

var clientOnce sync.Once

// Session - Database session
//
// 惰性初始化带显式替换保护：测试先把 Client 换成临时库再触发首条 SQL 时，
// clientOnce 一旦在此刻才触发会用真实 root dir 的连接覆盖替换（曾把测试注册的
// beacon 写进生产 ~/.sliver/sliver.db）。Client 已非空时跳过 once，尊重替换。
func Session() *gorm.DB {
	if Client == nil {
		clientOnce.Do(func() {
			if Client == nil {
				Client = newDBClient()
			}
		})
	}
	return Client.Session(&gorm.Session{
		FullSaveAssociations: true,
	})
}

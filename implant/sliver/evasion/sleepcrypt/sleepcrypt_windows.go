//go:build windows

package sleepcrypt

// Windows 实现：ntdll RtlEncryptMemory / RtlDecryptMemory。
// OptionFlags = RTL_ENCRYPT_MEMORY_SAME_PROCESS(0)：同进程内可解，密钥由内核
// 按进程派生（重启失效），攻击者需读进程内存 + 破坏系统密钥派生才能还原——
// 相比明文驻留，把「读内存即得」抬升为「需 R-2.2 级对抗」。
// MemorySize 必须为 RTL_ENCRYPT_MEMORY_SIZE(8) 的倍数。

import (
	"encoding/binary"
	"errors"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	rtlAlign         = 8
	rtlSameProcess   = 0x0000
	rtlStatusSuccess = 0x00000000
	lengthHeaderSize = 8
)

var (
	ntdll                = windows.NewLazyDLL("ntdll.dll")
	procRtlEncryptMemory = ntdll.NewProc("RtlEncryptMemory")
	procRtlDecryptMemory = ntdll.NewProc("RtlDecryptMemory")
)

func seal(plain []byte) ([]byte, error) {
	if plain == nil {
		return nil, errors.New("sleepcrypt: nil plaintext")
	}
	buf := pad(plain)
	if err := rtlCrypt(procRtlEncryptMemory, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

func unseal(cipher []byte) ([]byte, error) {
	if len(cipher) < lengthHeaderSize+rtlAlign || len(cipher)%rtlAlign != 0 {
		return nil, errors.New("sleepcrypt: invalid ciphertext size")
	}
	buf := make([]byte, len(cipher))
	copy(buf, cipher)
	if err := rtlCrypt(procRtlDecryptMemory, buf); err != nil {
		return nil, err
	}
	n := binary.LittleEndian.Uint64(buf)
	if n == 0 || n > uint64(len(buf)-lengthHeaderSize) {
		return nil, errors.New("sleepcrypt: corrupt length header")
	}
	out := make([]byte, n)
	copy(out, buf[lengthHeaderSize:lengthHeaderSize+int(n)])
	return out, nil
}

// pad 8 字节长度头（原始明文长度，LE）+ 明文 + 填充，总长 8 对齐。
func pad(plain []byte) []byte {
	total := len(plain) + lengthHeaderSize
	if rem := total % rtlAlign; rem != 0 {
		total += rtlAlign - rem
	}
	buf := make([]byte, total)
	binary.LittleEndian.PutUint64(buf, uint64(len(plain)))
	copy(buf[lengthHeaderSize:], plain)
	return buf
}

func rtlCrypt(proc *windows.LazyProc, buf []byte) error {
	nt, _, _ := proc.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), uintptr(rtlSameProcess))
	if nt != uintptr(rtlStatusSuccess) {
		return fmt.Errorf("sleepcrypt: rtl memory op ntstatus=0x%x", nt)
	}
	return nil
}

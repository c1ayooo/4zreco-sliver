//go:build !windows

package sleepcrypt

// 非 windows 平台：无 ntlll RtlEncryptMemory 等价机制，R-1a v1 不实现
// （G-3 动态决策下 linux 免杀压力低；云工作负载 EDR 由决策链降档）。调用方回落不加密路径。

func seal(plain []byte) ([]byte, error) { return nil, ErrUnsupported }

func unseal(cipher []byte) ([]byte, error) { return nil, ErrUnsupported }

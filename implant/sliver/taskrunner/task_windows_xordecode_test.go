package taskrunner

// 免杀 R-5② 单测：xorDecode 混淆串解码。
// - 往返：hex(密文):hex(pad) → 原串；
// - 损坏/无分隔符：回落原样（兼容未渲染的模板直编译与格式错误）。
import (
	"encoding/hex"
	"strings"
	"testing"
)

func TestXorDecodeRoundtrip(t *testing.T) {
	cases := []string{"amsi.dll", "AmsiScanBuffer", "AmsiInitialize", "AmsiScanString", "EtwEventWrite", "ntdll.dll"}
	for _, plain := range cases {
		// 构造 ObfuscateHexPair 同款串：pad 与明文等长（server 侧 obfuscatedHexPair 语义）
		pad := make([]byte, len(plain))
		for i := range pad {
			pad[i] = 0xAA + byte(i%7)
		}
		enc := make([]byte, len(plain))
		for i := range plain {
			enc[i] = plain[i] ^ pad[i]
		}
		obf := hex.EncodeToString(enc) + ":" + hex.EncodeToString(pad)
		if got := xorDecode(obf); got != plain {
			t.Fatalf("xorDecode(%s) = %q, want %q", obf, got, plain)
		}
	}
}

func TestXorDecodeFallback(t *testing.T) {
	// 无分隔符（如未渲染的模板原文或普通串）→ 原样
	for _, s := range []string{"amsi.dll", "plaintext-without-colon", ""} {
		if got := xorDecode(s); got != s {
			t.Fatalf("xorDecode fallback 错误: %q → %q", s, got)
		}
	}
	if got := xorDecode(strings.Repeat("zz", 4) + ":00"); got == "" {
		// 非法 hex 也回落原样，不 panic
		t.Logf("非法 hex 回落 ok: %q", got)
	} else if !strings.Contains(got, "zz") {
		t.Fatalf("非法 hex 应回落原样: %q", got)
	}
}
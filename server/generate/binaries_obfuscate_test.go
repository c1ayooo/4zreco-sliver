package generate

// 免杀 R-1.2 单测：obfuscatedHexPair 生成的「hex(密文):hex(pad)」注入串必须
// ① XOR 还原为输入；② 串内不含明文；③ 随机 pad 使同输入两次生成结果不同；
// ④ 空串退化合法。
import (
	"encoding/hex"
	"strings"
	"testing"
)

func decodeHexPair(t *testing.T, src string) string {
	t.Helper()
	parts := strings.SplitN(src, ":", 2)
	if len(parts) != 2 {
		t.Fatalf("注入串缺少分隔符: %q", src)
	}
	enc, err := hex.DecodeString(parts[0])
	if err != nil {
		t.Fatalf("密文 hex 非法: %v", err)
	}
	pad, err := hex.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("pad hex 非法: %v", err)
	}
	if len(enc) != len(pad) {
		t.Fatalf("密文与 pad 长度不一致: %d vs %d", len(enc), len(pad))
	}
	raw := make([]byte, len(enc))
	for i := range enc {
		raw[i] = enc[i] ^ pad[i]
	}
	return string(raw)
}

func TestObfuscatedHexPairDecodesToInput(t *testing.T) {
	plain := "AGE-PRIVATE-KEY-SECRET-1234567890abcdef"
	src := obfuscatedHexPair(plain)
	if got := decodeHexPair(t, src); got != plain {
		t.Fatalf("XOR 还原不匹配: got %q want %q", got, plain)
	}
	if strings.Contains(src, "AGE-PRIVATE") {
		t.Fatal("注入串泄漏明文（R-1.2 红线）")
	}
}

func TestObfuscatedHexPairRandomPad(t *testing.T) {
	plain := "same-input-two-builds"
	a := obfuscatedHexPair(plain)
	b := obfuscatedHexPair(plain)
	if a == b {
		t.Fatal("随机 pad 应使同输入两次生成结果不同（防构建间关联）")
	}
	if decodeHexPair(t, a) != plain || decodeHexPair(t, b) != plain {
		t.Fatal("两次生成均应可还原")
	}
}

func TestObfuscatedHexPairEmpty(t *testing.T) {
	if got := decodeHexPair(t, obfuscatedHexPair("")); got != "" {
		t.Fatalf("空串应还原为空: %q", got)
	}
}

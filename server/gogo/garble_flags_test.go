package gogo

// 免杀 S-2 单测：garble 参数默认集 / 环境变量覆盖 / 空值回落。
import "testing"

func TestGarbleFlagSetDefault(t *testing.T) {
	t.Setenv("SLIVER_GARBLE_FLAGS", "")
	flags := garbleFlagSet()
	want := []string{"-seed=random", "-literals", "-tiny"}
	if len(flags) != len(want) {
		t.Fatalf("默认参数集不符: %v", flags)
	}
	for i := range want {
		if flags[i] != want[i] {
			t.Fatalf("默认参数集不符: %v", flags)
		}
	}
}

func TestGarbleFlagSetOverride(t *testing.T) {
	t.Setenv("SLIVER_GARBLE_FLAGS", "-seed=abc -literals -tiny -controlflow")
	flags := garbleFlagSet()
	if len(flags) != 4 || flags[0] != "-seed=abc" || flags[3] != "-controlflow" {
		t.Fatalf("覆盖参数集不符: %v", flags)
	}
}

func TestGarbleFlagSetBlankFallback(t *testing.T) {
	t.Setenv("SLIVER_GARBLE_FLAGS", "   ")
	flags := garbleFlagSet()
	if len(flags) != 3 || flags[0] != "-seed=random" {
		t.Fatalf("空白值应回落默认集: %v", flags)
	}
}

package certs

// 免杀 C-1 单测：证书 Subject 词表自有化与地理一致性。
// ① orgSuffix 与国家联动（消 "Foo GmbH" + 加州地址类错配）；
// ② orgName 词源全部来自自有词表（弃 sliver 公开 codenames）；
// ③ postal per-country 格式正确（US 5 位 / CA 字数混合 / JP 7 位）；
// ④ province/locality/street 与国家同树（同国一致既有保证回归锁定）。
import (
	"regexp"
	"strings"
	"testing"
)

func TestOrgSuffixCountryBinding(t *testing.T) {
	// JP 组织名不得出现美式/德式后缀；反之 US 不得出现 K.K.
	for i := 0; i < 300; i++ {
		jpName := randomOrganization("JP")[0]
		if strings.Contains(jpName, "Inc.") || strings.Contains(jpName, "LLC") || strings.Contains(jpName, "GmbH") {
			t.Fatalf("JP 组织名泄漏他国后缀: %q", jpName)
		}
		usName := randomOrganization("US")[0]
		if strings.Contains(usName, "K.K.") || strings.Contains(usName, "GmbH") {
			t.Fatalf("US 组织名泄漏他国后缀: %q", usName)
		}
	}
}

func TestOrgNameWordsFromOwnCorpus(t *testing.T) {
	allowed := map[string]bool{}
	for _, w := range orgWordModifiers {
		allowed[strings.ToLower(w)] = true
	}
	for _, w := range orgWordNouns {
		allowed[strings.ToLower(w)] = true
	}
	for _, sfx := range append(append([]string{}, orgSuffixesByCountry["US"]...), orgSuffixesNeutral...) {
		for _, w := range strings.Fields(strings.ToLower(sfx)) {
			allowed[strings.TrimSuffix(w, ".")] = true
		}
	}
	// 组装变体可能整名大写/小写化，统一小写后按词切分校验词源
	for i := 0; i < 300; i++ {
		name := strings.ToLower(randomOrganization("US")[0])
		name = strings.NewReplacer(",", " ", "-", " ", ".", " ").Replace(name)
		for _, w := range strings.Fields(name) {
			if w == "" {
				continue
			}
			if !allowed[w] {
				// 允许复数形态（logistics 等本身收尾 s）与后缀词
				if strings.HasSuffix(w, "s") && allowed[strings.TrimSuffix(w, "s")] {
					continue
				}
				t.Fatalf("词 %q 不在自有词表（疑似 codenames 回潮）: %q", w, name)
			}
		}
	}
}

func TestPostalCodePerCountry(t *testing.T) {
	usRe := regexp.MustCompile(`^\d{5}$`)
	caRe := regexp.MustCompile(`^[ABHLMNKGJPRSTVYXabhlmnkgjprstvyx]\d[ABHLMNKGJPRSTVYXabhlmnkgjprstvyx]$`)
	jpHyphenRe := regexp.MustCompile(`^\d{3}-\d{4}$`)
	jpDigitsRe := regexp.MustCompile(`^\d{7}$`)
	for i := 0; i < 500; i++ {
		for _, code := range randomPostalCode([]string{"US"}) {
			if !usRe.MatchString(code) {
				t.Fatalf("US 邮编格式不符: %q", code)
			}
		}
		for _, code := range randomPostalCode([]string{"CA"}) {
			if !caRe.MatchString(code) {
				t.Fatalf("CA 邮编格式不符: %q", code)
			}
		}
		for _, code := range randomPostalCode([]string{"JP"}) {
			if !jpHyphenRe.MatchString(code) && !jpDigitsRe.MatchString(code) {
				t.Fatalf("JP 邮编格式不符: %q", code)
			}
		}
	}
}

func TestSubjectGeographicConsistency(t *testing.T) {
	for i := 0; i < 300; i++ {
		country, province, _, _ := randomProvinceLocalityStreetAddress()
		c := country[0]
		if _, ok := subjects[c]; !ok {
			t.Fatalf("未知国家: %q", c)
		}
		if _, ok := subjects[c][province[0]]; !ok {
			t.Fatalf("省/州 %q 不在国家 %q 的地址树内（地理错配回归）", province[0], c)
		}
	}
}

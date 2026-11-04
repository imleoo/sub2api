package tlsfingerprint

import (
	"crypto/md5"
	"fmt"
	"strconv"
	"testing"

	utls "github.com/refraction-networking/utls"
)

// capturedCodexCipherSuites 等常量为 2026-11-04 实测 codex-cli 0.163.0
// (x86_64-unknown-linux-musl) ClientHello 的十进制值，此处逐项校验 profile 复现。
var (
	capturedCodexCipherSuites = []uint16{4866, 4867, 4865, 49196, 49200, 159, 52393, 52392, 52394, 49195, 49199, 158, 49188, 49192, 107, 49187, 49191, 103, 49162, 49172, 57, 49161, 49171, 51, 157, 156, 61, 60, 53, 47}
	capturedCodexExtensions   = []uint16{65281, 0, 11, 10, 35, 22, 23, 13, 43, 45, 51}
	capturedCodexCurves       = []uint16{4588, 29, 23, 30, 24, 25, 256, 257}
	capturedCodexKeyShares    = []uint16{4588, 29}
	capturedCodexSigAlgs      = []uint16{2309, 2310, 2308, 1027, 1283, 1539, 2055, 2056, 2074, 2075, 2076, 2057, 2058, 2059, 2052, 2053, 2054, 1025, 1281, 1537, 771, 769, 770, 1026, 1282, 1538}
)

// capturedCodexJA3 实测 codex-cli 0.163.0（x86_64-unknown-linux-musl，
// native-tls/OpenSSL）ClientHello 的 JA3 哈希。profile 字段逐项复现该指纹，
// 并已通过本地 capture server 线上回环验证（emitted hello JA3 与真实客户端一致）。
const capturedCodexJA3 = "0b85eb0d4981e69064e40753e4f0ac5f"

func TestOpenSSLNativeProfileJA3MatchesCodex(t *testing.T) {
	// 按 JA3 规范从 profile 生效字段计算（GREASE 已剔除，本 profile 无 GREASE）。
	p := OpenSSLNativeProfile()
	ja3 := fmt.Sprintf("771,%s,%s,%s,%s",
		joinDecForTest(p.CipherSuites),
		joinDecForTest(p.Extensions),
		joinDecForTest(p.Curves),
		joinDecForTest(p.PointFormats))
	hash := fmt.Sprintf("%x", md5.Sum([]byte(ja3)))
	if hash != capturedCodexJA3 {
		t.Fatalf("profile JA3 mismatch:\n got:  %s (%s)\n want: %s", hash, ja3, capturedCodexJA3)
	}
}

func joinDecForTest(vals []uint16) string {
	out := make([]byte, 0, len(vals)*4)
	for i, v := range vals {
		if i > 0 {
			out = append(out, '-')
		}
		out = append(out, []byte(strconv.Itoa(int(v)))...)
	}
	return string(out)
}

func TestOpenSSLNativeProfileMatchesCapture(t *testing.T) {
	p := OpenSSLNativeProfile()
	if p.Name != ProfileNameOpenSSLNative {
		t.Fatalf("profile name mismatch: %s", p.Name)
	}
	assertUint16Equal(t, "cipher_suites", capturedCodexCipherSuites, p.CipherSuites)
	assertUint16Equal(t, "extensions", capturedCodexExtensions, p.Extensions)
	assertUint16Equal(t, "curves", capturedCodexCurves, p.Curves)
	assertUint16Equal(t, "key_share_groups", capturedCodexKeyShares, p.KeyShareGroups)
	assertUint16Equal(t, "signature_algorithms", capturedCodexSigAlgs, p.SignatureAlgorithms)
	if p.EnableGREASE {
		t.Fatal("captured codex hello carries no GREASE")
	}
	// 无 ALPN：真实 codex 不发 ALPN 扩展（协议层 HTTP/1.1）。
	if len(p.ALPNProtocols) != 0 {
		t.Fatalf("profile must not set ALPN, got %v", p.ALPNProtocols)
	}
	if len(p.SupportedVersions) != 2 || p.SupportedVersions[0] != 0x0304 || p.SupportedVersions[1] != 0x0303 {
		t.Fatalf("supported_versions mismatch: %v", p.SupportedVersions)
	}
	if len(p.PSKModes) != 1 || p.PSKModes[0] != 1 {
		t.Fatalf("psk_modes mismatch: %v", p.PSKModes)
	}
}

func TestOpenSSLNativeProfileBuildsSpec(t *testing.T) {
	spec := buildClientHelloSpecFromProfile(OpenSSLNativeProfile())
	if spec == nil {
		t.Fatal("spec build failed")
	}
	// 无 GREASE
	for _, ext := range spec.Extensions {
		if _, ok := ext.(*utls.UtlsGREASEExtension); ok {
			t.Fatal("built spec must not contain GREASE extensions")
		}
		if _, ok := ext.(*utls.GREASEEncryptedClientHelloExtension); ok {
			t.Fatal("built spec must not contain ECH GREASE extension")
		}
	}
	// 扩展集合与顺序（剔除 GREASE 后逐项比对）
	got := make([]uint16, 0, len(spec.Extensions))
	for _, ext := range spec.Extensions {
		got = append(got, extensionIDForTest(ext))
	}
	assertUint16Equal(t, "spec extensions (order)", capturedCodexExtensions, got)
	// 套件顺序
	if len(spec.CipherSuites) != len(capturedCodexCipherSuites) {
		t.Fatalf("spec cipher count mismatch: got %d", len(spec.CipherSuites))
	}
	for i, cs := range spec.CipherSuites {
		if uint16(cs) != capturedCodexCipherSuites[i] {
			t.Fatalf("spec cipher[%d]: got 0x%04x, want 0x%04x", i, cs, capturedCodexCipherSuites[i])
		}
	}
	// 无 ALPN 扩展
	for _, ext := range spec.Extensions {
		if _, ok := ext.(*utls.ALPNExtension); ok {
			t.Fatal("built spec must not contain ALPN extension")
		}
	}
}

func TestProfileForHTTP1OnlyOverridesALPN(t *testing.T) {
	// 用带 h2 ALPN 的模板验证覆写语义（DB 模板场景）。
	src := &Profile{
		Name:          "h2-template",
		ALPNProtocols: []string{"h2", "http/1.1"},
		Extensions:    []uint16{0, 16, 43, 51},
	}
	h1 := ProfileForHTTP1Only(src)
	if len(h1.ALPNProtocols) != 1 || h1.ALPNProtocols[0] != "http/1.1" {
		t.Fatalf("expected ALPN http/1.1 only, got %v", h1.ALPNProtocols)
	}
	if len(src.ALPNProtocols) != 2 {
		t.Fatalf("source profile ALPN mutated: %v", src.ALPNProtocols)
	}
	// nil 容错
	if ProfileForHTTP1Only(nil) != nil {
		t.Fatal("nil profile should return nil")
	}
	// 无 ALPN 的 profile 拷贝后仍无 ALPN
	if got := ProfileForHTTP1Only(OpenSSLNativeProfile()); len(got.ALPNProtocols) != 1 {
		t.Fatalf("expected cloned profile with h1-only ALPN, got %v", got.ALPNProtocols)
	}
}

func extensionIDForTest(ext utls.TLSExtension) uint16 {
	switch e := ext.(type) {
	case *utls.SNIExtension:
		return 0
	case *utls.StatusRequestExtension:
		return 5
	case *utls.SupportedCurvesExtension:
		return 10
	case *utls.SupportedPointsExtension:
		return 11
	case *utls.SignatureAlgorithmsExtension:
		return 13
	case *utls.ALPNExtension:
		return 16
	case *utls.GenericExtension:
		return e.Id
	case *utls.SCTExtension:
		return 18
	case *utls.ExtendedMasterSecretExtension:
		return 23
	case *utls.SessionTicketExtension:
		return 35
	case *utls.SupportedVersionsExtension:
		return 43
	case *utls.PSKKeyExchangeModesExtension:
		return 45
	case *utls.KeyShareExtension:
		return 51
	case *utls.RenegotiationInfoExtension:
		return 65281
	case *utls.UtlsGREASEExtension:
		return 0x0a0a
	case *utls.GREASEEncryptedClientHelloExtension:
		return 65037
	default:
		return 0xffff
	}
}

func assertUint16Equal(t *testing.T, name string, expected, actual []uint16) {
	t.Helper()
	if len(expected) != len(actual) {
		t.Fatalf("%s: length mismatch: got %d want %d\n got:  %v\n want: %v", name, len(actual), len(expected), actual, expected)
	}
	for i := range expected {
		if expected[i] != actual[i] {
			t.Fatalf("%s[%d]: got %d (0x%04x), want %d (0x%04x)", name, i, actual[i], actual[i], expected[i], expected[i])
		}
	}
}

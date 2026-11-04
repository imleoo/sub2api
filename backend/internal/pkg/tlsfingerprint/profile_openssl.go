package tlsfingerprint

// OpenSSL(native-tls) 风格指纹 —— 模拟官方 Codex CLI 的真实出站 ClientHello。
//
// 参数来源：2026-11-04 本地 capture server 实测 codex-cli 0.163.0
// （x86_64-unknown-linux-musl 官方发布版，在 Docker/debian 内发起对自建
// HTTPS 端点的请求，握手前截获 ClientHello）。
//
// 关键事实（源码佐证）：codex-rs 的 reqwest 未启用 rustls-tls feature
// （workspace Cargo.toml: `reqwest = { version = "0.12", features = ["cookies"] }`），
// 走默认 native-tls —— Linux 上即 OpenSSL。因此 UA 声称 Ubuntu 的 sub2api
// 出站流量应模拟 OpenSSL 指纹，而非 rustls。
//
// 实测指纹（JA3 0b85eb0d4981e69064e40753e4f0ac5f）：
//   - 30 个套件，OpenSSL 3.x 默认序（TLS1.3: 1302,1303,1301 打头）
//   - 扩展序：renego(65281) 在 SNI(0) 之前，含 encrypt_then_mac(22)，
//     无 ECH(65037)、无 GREASE、无 SCT(18)、无 OCSP(5)
//   - 曲线含 X25519MLKEM768(0x11EC) 后量子混合组，key_share 发送
//     [X25519MLKEM768, X25519] 两组
//   - 签名算法 26 项（含 ML-DSA 0x0904-0x0906、ed25519/ed448）
//   - supported_versions [1.3, 1.2]，psk_modes [dhe_ke]
//   - 无 ALPN 扩展 —— 真实 codex 出站走 HTTP/1.1（不要加 h2 ALPN，
//     否则协商出 h2 会与真实客户端的协议层行为背离）
//
// 各平台差异：macOS 官方构建走 SecureTransport（TLS1.2-only、无 ALPN、
// 无 TLS1.3 扩展，JA3 e4d448cdfe06dc1243c1eb026c74ac9a）；Linux/CI 场景
// 与 sub2api 的 Ubuntu UA 声明一致，故采用本 OpenSSL 指纹。
const ProfileNameOpenSSLNative = "openssl-native-tls"

// opensslCipherSuites 实测 codex(OpenSSL) 套件及顺序。
var opensslCipherSuites = []uint16{
	0x1302, // TLS_AES_256_GCM_SHA384
	0x1303, // TLS_CHACHA20_POLY1305_SHA256
	0x1301, // TLS_AES_128_GCM_SHA256
	0xc02c, // ECDHE_ECDSA_AES256_GCM_SHA384
	0xc030, // ECDHE_RSA_AES256_GCM_SHA384
	0x009f, // DHE_RSA_AES256_GCM_SHA384
	0xcca9, // ECDHE_ECDSA_CHACHA20_POLY1305
	0xcca8, // ECDHE_RSA_CHACHA20_POLY1305
	0xccaa, // DHE_RSA_CHACHA20_POLY1305
	0xc02b, // ECDHE_ECDSA_AES128_GCM_SHA256
	0xc02f, // ECDHE_RSA_AES128_GCM_SHA256
	0x009e, // DHE_RSA_AES128_GCM_SHA256
	0xc024, // ECDHE_ECDSA_AES256_SHA384
	0xc028, // ECDHE_RSA_AES256_SHA384
	0x006b, // DHE_RSA_AES256_SHA256
	0xc023, // ECDHE_ECDSA_AES128_SHA256
	0xc027, // ECDHE_RSA_AES128_SHA256
	0x0067, // DHE_RSA_AES128_SHA256
	0xc00a, // ECDHE_ECDSA_AES256_SHA
	0xc014, // ECDHE_RSA_AES256_SHA
	0x0039, // DHE_RSA_AES256_SHA
	0xc009, // ECDHE_ECDSA_AES128_SHA
	0xc013, // ECDHE_RSA_AES128_SHA
	0x0033, // DHE_RSA_AES128_SHA
	0x009d, // RSA_AES256_GCM_SHA384
	0x009c, // RSA_AES128_GCM_SHA256
	0x003d, // RSA_AES256_SHA256
	0x003c, // RSA_AES128_SHA256
	0x0035, // RSA_AES256_SHA
	0x002f, // RSA_AES128_SHA
}

// opensslExtensionOrder 实测扩展顺序（renego 在 SNI 之前；encrypt_then_mac
// 为空载荷扩展，由 GenericExtension 原样发出；无 ALPN —— 与真实客户端一致）。
var opensslExtensionOrder = []uint16{
	65281, // renegotiation_info
	0,     // server_name
	11,    // ec_point_formats
	10,    // supported_groups
	35,    // session_ticket
	22,    // encrypt_then_mac
	23,    // extended_master_secret
	13,    // signature_algorithms
	43,    // supported_versions
	45,    // psk_key_exchange_modes
	51,    // key_share
}

// opensslCurves 实测命名群组（含后量子混合组 X25519MLKEM768 与 FFDHE）。
var opensslCurves = []uint16{
	4588, // X25519MLKEM768 (0x11EC)
	29,   // X25519
	23,   // secp256r1
	30,   // X448
	24,   // secp384r1
	25,   // secp521r1
	256,  // ffdhe2048
	257,  // ffdhe3072
}

// opensslSignatureAlgorithms 实测签名算法（含 ML-DSA 与 ed25519/ed448）。
var opensslSignatureAlgorithms = []uint16{
	2309, // 0x0905 ML-DSA-65
	2310, // 0x0906 ML-DSA-87
	2308, // 0x0904 ML-DSA-44
	1027, // ecdsa_secp256r1_sha256
	1283, // ecdsa_secp384r1_sha384
	1539, // ecdsa_secp521r1_sha512
	2055, // ed25519
	2056, // ed448
	2074, // 实测原样保留
	2075, // 实测原样保留
	2076, // 实测原样保留
	2057, // rsa_pss_pss_sha256
	2058, // rsa_pss_pss_sha384
	2059, // rsa_pss_pss_sha512
	2052, // rsa_pss_rsae_sha256
	2053, // rsa_pss_rsae_sha384
	2054, // rsa_pss_rsae_sha512
	1025, // rsa_pkcs1_sha256
	1281, // rsa_pkcs1_sha384
	1537, // rsa_pkcs1_sha512
	771,  // 实测原样保留
	769,  // 实测原样保留
	770,  // 实测原样保留
	1026, // ecdsa_sha1
	1282, // 实测原样保留
	1538, // 实测原样保留
}

// OpenSSLNativeProfile 返回模拟官方 Codex CLI（native-tls/OpenSSL）的指纹 Profile。
// 无 ALPN：协议层为 HTTP/1.1，与真实客户端一致。
func OpenSSLNativeProfile() *Profile {
	return &Profile{
		Name:                ProfileNameOpenSSLNative,
		CipherSuites:        cloneUint16s(opensslCipherSuites),
		Curves:              cloneUint16s(opensslCurves),
		PointFormats:        []uint16{0},
		EnableGREASE:        false,
		SignatureAlgorithms: cloneUint16s(opensslSignatureAlgorithms),
		ALPNProtocols:       nil, // 无 ALPN 扩展（真实客户端不发）
		SupportedVersions:   []uint16{0x0304, 0x0303},
		KeyShareGroups:      []uint16{4588, 29},
		PSKModes:            []uint16{1},
		Extensions:          cloneUint16s(opensslExtensionOrder),
	}
}

// ProfileForHTTP1Only 返回 ALPN 覆写为仅 ["http/1.1"] 的 profile 副本，
// 供配置了 h2 ALPN 的模板在 H1-only 传输上使用（避免协商出 h2 却跑 HTTP/1.1）。
// 对本身无 ALPN 的 profile（如 OpenSSLNativeProfile）为透传拷贝。
func ProfileForHTTP1Only(p *Profile) *Profile {
	if p == nil {
		return nil
	}
	clone := *p
	clone.CipherSuites = cloneUint16s(p.CipherSuites)
	clone.Curves = cloneUint16s(p.Curves)
	clone.PointFormats = cloneUint16s(p.PointFormats)
	clone.SignatureAlgorithms = cloneUint16s(p.SignatureAlgorithms)
	clone.ALPNProtocols = []string{"http/1.1"}
	clone.SupportedVersions = cloneUint16s(p.SupportedVersions)
	clone.KeyShareGroups = cloneUint16s(p.KeyShareGroups)
	clone.PSKModes = cloneUint16s(p.PSKModes)
	clone.Extensions = cloneUint16s(p.Extensions)
	return &clone
}

func cloneUint16s(vals []uint16) []uint16 {
	if len(vals) == 0 {
		return nil
	}
	out := make([]uint16, len(vals))
	copy(out, vals)
	return out
}

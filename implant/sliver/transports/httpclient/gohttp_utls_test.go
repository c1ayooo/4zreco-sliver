package httpclient

// 免杀 R-7 单测：utls DialTLS 闭包 ↔ Go 标准 crypto/tls 服务端互操作。
// sliver server 就是 Go 标准库 TLS 服务端——本测试即服务端兼容性回归：
// 握手成功、ALPN 协商 http/1.1（v1 口径）、TLS ≥1.2、数据面可用。
import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"testing"
	"time"

	utls "github.com/refraction-networking/utls"
)

func testTLSServer(t *testing.T) net.Listener {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "c2.local"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		DNSNames:     []string{"c2.local"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert := tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
	listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{"h2", "http/1.1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				tlsConn := conn.(*tls.Conn)
				_ = tlsConn.HandshakeContext(context.Background())
				buf := make([]byte, 64)
				for {
					if n, err := tlsConn.Read(buf); err != nil || n == 0 {
						tlsConn.Close()
						return
					}
				}
			}()
		}
	}()
	return listener
}

func TestUtlsDialContextInterop(t *testing.T) {
	listener := testTLSServer(t)

	closure := utlsDialContext(&tls.Config{InsecureSkipVerify: true}, 5*time.Second)
	conn, err := closure(context.Background(), "tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("utls 握手失败（server 兼容性回归）: %v", err)
	}
	defer conn.Close()

	uConn := conn.(*utls.UConn)
	state := uConn.ConnectionState()
	if state.NegotiatedProtocol != "http/1.1" {
		t.Fatalf("ALPN 口径应为 http/1.1（v1 server 兼容优先）: %q", state.NegotiatedProtocol)
	}
	if state.Version < utls.VersionTLS12 {
		t.Fatalf("TLS 版本过低: 0x%x", state.Version)
	}

	// 数据面：写出即证明协商后的连接可用于 HTTP 收发
	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if _, err := conn.Write([]byte("PING")); err != nil {
		t.Fatalf("数据面不可用: %v", err)
	}
}

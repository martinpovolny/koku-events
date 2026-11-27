//go:build !rest_watch

package osac

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"log/slog"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// TestWatchEvents_ConnectsWithoutH2ALPN verifies that upgrading grpc-go >= 1.67
// preserves TLS connectivity to servers (such as OSAC fulfillment-service /
// OpenShift routers) that do not negotiate "h2" in TLS ALPN.
func TestWatchEvents_ConnectsWithoutH2ALPN(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "localhost"},
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	caFile := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(caFile, certPEM, 0o600); err != nil {
		t.Fatalf("write ca file: %v", err)
	}

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("load keypair: %v", err)
	}

	// Deliberately omit "h2" from NextProtos to simulate OSAC / OpenShift router
	// TLS listeners that complete TLS without ALPN negotiation.
	rawLis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	tlsLis := tls.NewListener(rawLis, &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		MinVersion:   tls.VersionTLS12,
		NextProtos:   nil,
	})

	srv := grpc.NewServer()
	reflection.Register(srv)
	go func() { _ = srv.Serve(tlsLis) }()
	defer srv.Stop()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	client, err := NewClient("https://"+rawLis.Addr().String(), "", caFile, logger)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	client.SetGRPCAddress(rawLis.Addr().String())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// The reflection call should complete the TLS handshake (no ALPN error) and
	// fail only because our test server does not register osac.public.v1.Events.
	err = client.WatchEvents(ctx, func(Event) error { return nil })
	if err == nil {
		t.Fatal("expected missing-service reflection error, got nil")
	}
	if strings.Contains(strings.ToLower(err.Error()), "alpn") {
		t.Fatalf("WatchEvents failed with ALPN error after grpc-go upgrade: %v", err)
	}
	if !strings.Contains(err.Error(), "grpc resolve service") {
		t.Fatalf("expected error after TLS handshake during reflection lookup, got: %v", err)
	}
}

package database

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	"vertexia-frontend/backend/config"
)

func testCAPEM(t *testing.T) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

func baseConfig() *config.Config {
	return &config.Config{DBUser: "app", DBPass: "p@ss:word", DBHost: "db.example.com", DBPort: "3306", DBName: "vertexia", DBTimeout: 5 * time.Second}
}

func TestBuildDSNWithoutTLS(t *testing.T) {
	dsn, err := BuildDSN(baseConfig())
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Passwd != "p@ss:word" || parsed.Addr != "db.example.com:3306" || !parsed.ParseTime || parsed.Timeout != 5*time.Second {
		t.Fatalf("unexpected DSN fields: %+v", parsed)
	}
	if parsed.TLS != nil {
		t.Fatal("TLS must be off by default")
	}
}

func TestBuildDSNVerifiedTLSWithCustomCA(t *testing.T) {
	cfg := baseConfig()
	cfg.DBTLS = "true"
	// Secret stores often flatten newlines into literal "\n".
	cfg.DBTLSCAPEM = strings.ReplaceAll(testCAPEM(t), "\n", `\n`)
	cfg.DBTLSServerName = "mysql.internal"
	dsn, err := BuildDSN(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(dsn, "tls="+tlsConfigName) {
		t.Fatalf("custom TLS config not referenced: %s", dsn)
	}
	parsed, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.TLS == nil || parsed.TLS.InsecureSkipVerify || parsed.TLS.ServerName != "mysql.internal" || parsed.TLS.RootCAs == nil {
		t.Fatalf("TLS config does not verify certificates: %+v", parsed.TLS)
	}
}

func TestBuildDSNSystemRootsTLS(t *testing.T) {
	cfg := baseConfig()
	cfg.DBTLS = "true"
	dsn, err := BuildDSN(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(dsn, "tls=true") {
		t.Fatalf("expected tls=true, got %s", dsn)
	}
}

func TestBuildDSNRejectsBadCA(t *testing.T) {
	cfg := baseConfig()
	cfg.DBTLS = "true"
	cfg.DBTLSCAPEM = "not a certificate"
	if _, err := BuildDSN(cfg); err == nil {
		t.Fatal("expected an error for an invalid CA bundle")
	}
}

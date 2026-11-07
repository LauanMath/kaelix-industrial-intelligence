package secure

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestCertificateIsP256AndReused(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "data")
	first, fingerprint, err := LoadOrCreateCertificate(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(fingerprint) {
		t.Fatalf("fingerprint %q", fingerprint)
	}
	leaf, err := x509.ParseCertificate(first.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	if key, ok := leaf.PublicKey.(*ecdsa.PublicKey); !ok || key.Curve != elliptic.P256() {
		t.Fatalf("key %T", leaf.PublicKey)
	}
	info, err := os.Stat(filepath.Join(dir, PrivateKeyFile))
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("private key mode: %v %v", info, err)
	}
	_, again, err := LoadOrCreateCertificate(dir)
	if err != nil || again != fingerprint {
		t.Fatalf("certificate was not reused: %v %s %s", err, again, fingerprint)
	}
}

// Silently regenerating would change the fingerprint and lock out paired gateways.
func TestCertificateNotRegeneratedWhenHalfIsMissing(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := LoadOrCreateCertificate(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, PrivateKeyFile)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := LoadOrCreateCertificate(dir); err == nil {
		t.Fatal("regenerated over a missing key")
	}
}

func TestWebCertificateFitsApplePlatformsAndIsReused(t *testing.T) {
	dir := t.TempDir()
	first, err := LoadOrCreateWebCertificate(dir)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(first.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	if leaf.NotAfter.Sub(leaf.NotBefore) > 825*24*time.Hour {
		t.Fatalf("valid for %v", leaf.NotAfter.Sub(leaf.NotBefore))
	}
	if len(leaf.ExtKeyUsage) != 1 || leaf.ExtKeyUsage[0] != x509.ExtKeyUsageServerAuth || len(leaf.DNSNames) == 0 {
		t.Fatalf("usage %v names %v", leaf.ExtKeyUsage, leaf.DNSNames)
	}
	info, err := os.Stat(filepath.Join(dir, WebPrivateKeyFile))
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("private key mode: %v %v", info, err)
	}
	again, err := LoadOrCreateWebCertificate(dir)
	if err != nil || Fingerprint(again) != Fingerprint(first) {
		t.Fatalf("certificate was not reused: %v", err)
	}
	// It must never touch the pinned gateway pair.
	if _, err = os.Stat(filepath.Join(dir, CertificateFile)); !os.IsNotExist(err) {
		t.Fatalf("gateway certificate created: %v", err)
	}
}

func TestSecretKeyFileAndEncryption(t *testing.T) {
	dir := t.TempDir()
	box, err := LoadOrCreateBox(dir)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, SecretKeyFile)
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 || info.Size() != secretKeySize {
		t.Fatalf("key file: %+v %v", info, err)
	}
	const password = "senha-do-wifi"
	first, err := box.Seal([]byte(password), []byte("gateway/rede"))
	if err != nil {
		t.Fatal(err)
	}
	second, _ := box.Seal([]byte(password), []byte("gateway/rede"))
	if first == second {
		t.Fatal("nonce reused: equal passwords produced equal ciphertexts")
	}
	if strings.Contains(first, password) {
		t.Fatal("plaintext in the sealed value")
	}
	reopened, err := LoadOrCreateBox(dir)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := reopened.Open(first, []byte("gateway/rede"))
	if err != nil || !bytes.Equal(plain, []byte(password)) {
		t.Fatalf("key was not reused: %v", err)
	}
	if _, err = reopened.Open(first, []byte("outro/registro")); err == nil {
		t.Fatal("sealed value opened under different additional data")
	}
	other, err := NewBox(bytes.Repeat([]byte{7}, secretKeySize))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = other.Open(first, []byte("gateway/rede")); err == nil || strings.Contains(err.Error(), password) {
		t.Fatalf("wrong key: %v", err)
	}
	if _, err = other.Open("not base64!", nil); err == nil {
		t.Fatal("malformed value accepted")
	}
	// A widened mode is restored.
	if err = os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err = LoadOrCreateBox(dir); err != nil {
		t.Fatal(err)
	}
	if info, _ = os.Stat(path); info.Mode().Perm() != 0o600 {
		t.Fatalf("mode %o", info.Mode().Perm())
	}
	if err = os.WriteFile(path, []byte("curta"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err = LoadOrCreateBox(dir); err == nil {
		t.Fatal("short key accepted")
	}
}

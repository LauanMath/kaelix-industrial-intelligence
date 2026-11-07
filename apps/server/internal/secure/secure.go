// Package secure keeps the material that must survive restarts in API_DATA_DIR: the
// gateway listener certificate, pinned by the firmware, and the Wi-Fi password key.
package secure

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

const (
	CertificateFile = "gateway-tls.crt"
	PrivateKeyFile  = "gateway-tls.key"
	SecretKeyFile   = "secret.key"
	secretKeySize   = 32
	// Gateways pin the fingerprint and cannot renew by themselves.
	certificateLifetime = 20 * 365 * 24 * time.Hour

	WebCertificateFile = "web-tls.crt"
	WebPrivateKeyFile  = "web-tls.key"
	// Apple devices refuse a TLS certificate valid for more than 825 days.
	webCertificateLifetime = 800 * 24 * time.Hour
	webCertificateRenewal  = 30 * 24 * time.Hour
)

func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

// writeAtomic never leaves a partial credential behind after a crash.
func writeAtomic(path string, data []byte, mode os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err = tmp.Chmod(mode); err == nil {
		if _, err = tmp.Write(data); err == nil {
			err = tmp.Sync()
		}
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// LoadOrCreateCertificate returns the certificate in dir, generating it on the first run,
// and its fingerprint. A missing half of the pair is an error, not a reason to regenerate:
// a new certificate changes the fingerprint and locks every paired gateway out.
func LoadOrCreateCertificate(dir string) (tls.Certificate, string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return tls.Certificate{}, "", fmt.Errorf("data directory: %w", err)
	}
	certPath, keyPath := filepath.Join(dir, CertificateFile), filepath.Join(dir, PrivateKeyFile)
	hasCert, err := exists(certPath)
	if err != nil {
		return tls.Certificate{}, "", err
	}
	hasKey, err := exists(keyPath)
	if err != nil {
		return tls.Certificate{}, "", err
	}
	if hasCert != hasKey {
		return tls.Certificate{}, "", errors.New("only one of the TLS certificate and key exists in the data directory; restore the other or remove both to generate a new pair, then pair the gateways again")
	}
	if !hasCert {
		if err = generateCertificate(certPath, keyPath, certificateLifetime, []net.IP{net.IPv4(127, 0, 0, 1)}); err != nil {
			return tls.Certificate{}, "", err
		}
	}
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return tls.Certificate{}, "", fmt.Errorf("TLS certificate: %w", err)
	}
	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return tls.Certificate{}, "", fmt.Errorf("TLS certificate: %w", err)
	}
	if key, ok := leaf.PublicKey.(*ecdsa.PublicKey); !ok || key.Curve != elliptic.P256() {
		return tls.Certificate{}, "", errors.New("TLS certificate must use ECDSA P-256")
	}
	cert.Leaf = leaf
	return cert, Fingerprint(cert), nil
}

// Fingerprint is the lowercase SHA-256 of the DER certificate, the value gateways pin.
func Fingerprint(cert tls.Certificate) string {
	sum := sha256.Sum256(cert.Certificate[0])
	return hex.EncodeToString(sum[:])
}

// LoadOrCreateWebCertificate returns the certificate of the collection page. Nothing pins
// it, so unlike the gateway certificate it is simply replaced when unreadable or near expiry.
func LoadOrCreateWebCertificate(dir string) (tls.Certificate, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return tls.Certificate{}, fmt.Errorf("data directory: %w", err)
	}
	certPath, keyPath := filepath.Join(dir, WebCertificateFile), filepath.Join(dir, WebPrivateKeyFile)
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err == nil {
		leaf, parseErr := x509.ParseCertificate(cert.Certificate[0])
		if parseErr == nil && time.Now().Add(webCertificateRenewal).Before(leaf.NotAfter) {
			cert.Leaf = leaf
			return cert, nil
		}
	}
	if err = generateCertificate(certPath, keyPath, webCertificateLifetime, localAddresses()); err != nil {
		return tls.Certificate{}, err
	}
	return tls.LoadX509KeyPair(certPath, keyPath)
}

func localAddresses() []net.IP {
	ips := []net.IP{net.IPv4(127, 0, 0, 1)}
	addresses, err := net.InterfaceAddrs()
	if err != nil {
		return ips
	}
	for _, address := range addresses {
		if network, ok := address.(*net.IPNet); ok && network.IP.IsPrivate() {
			ips = append(ips, network.IP)
		}
	}
	return ips
}

func generateCertificate(certPath, keyPath string, lifetime time.Duration, ips []net.IP) error {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	names := []string{"kaelix.local", "localhost"}
	if host, err := os.Hostname(); err == nil && host != "" {
		names = append(names, host)
	}
	template := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: "Kaelix API"}, NotBefore: now.Add(-24 * time.Hour), NotAfter: now.Add(lifetime), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, BasicConstraintsValid: true, DNSNames: names, IPAddresses: ips}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return err
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return err
	}
	// The key goes first, so a crash between the writes is reported instead of hidden.
	if err = writeAtomic(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil {
		return err
	}
	return writeAtomic(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o644)
}

// Box encrypts small secrets with AES-256-GCM. The additional data binds a sealed value
// to the record it belongs to, so it cannot be copied to another one.
type Box struct{ aead cipher.AEAD }

// NewBox builds a Box from a 32-byte key.
func NewBox(key []byte) (*Box, error) {
	if len(key) != secretKeySize {
		return nil, fmt.Errorf("secret key must have %d bytes", secretKeySize)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Box{aead: aead}, nil
}

// LoadOrCreateBox reads API_DATA_DIR/secret.key, creating it with mode 0600 on the first
// run and restoring that mode if it was widened.
func LoadOrCreateBox(dir string) (*Box, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("data directory: %w", err)
	}
	path := filepath.Join(dir, SecretKeyFile)
	hasKey, err := exists(path)
	if err != nil {
		return nil, err
	}
	if !hasKey {
		key := make([]byte, secretKeySize)
		if _, err = rand.Read(key); err != nil {
			return nil, err
		}
		if err = writeAtomic(path, key, 0o600); err != nil {
			return nil, err
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode().Perm() != 0o600 {
		if err = os.Chmod(path, 0o600); err != nil {
			return nil, err
		}
	}
	key, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return NewBox(key)
}

// Seal returns base64(nonce || ciphertext).
func (b *Box) Seal(plaintext, additional []byte) (string, error) {
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b.aead.Seal(nonce, nonce, plaintext, additional)), nil
}

// Open reverses Seal. Its error never carries the content.
func (b *Box) Open(sealed string, additional []byte) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(sealed)
	if err != nil || len(raw) < b.aead.NonceSize() {
		return nil, errors.New("sealed value is malformed")
	}
	plain, err := b.aead.Open(nil, raw[:b.aead.NonceSize()], raw[b.aead.NonceSize():], additional)
	if err != nil {
		return nil, errors.New("sealed value cannot be opened with this key")
	}
	return plain, nil
}

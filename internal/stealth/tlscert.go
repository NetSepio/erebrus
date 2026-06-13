package stealth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"time"
)

// settings keys for the node's self-signed Hysteria2 TLS certificate.
const (
	keyHysteria2Cert = "stealth_hysteria2_cert_pem"
	keyHysteria2Key  = "stealth_hysteria2_key_pem"
)

// loadOrCreateCert returns the PEM cert/key pair the Hysteria2 (QUIC) carrier
// presents. Hysteria2 runs over real TLS 1.3, so unlike the VLESS carrier it
// cannot use REALITY; a long-lived self-signed cert is generated once and
// persisted. Clients pin nothing and connect with insecure verification — the
// actual VPN authentication lives in the inner WireGuard tunnel.
func loadOrCreateCert(ctx context.Context, st SettingsStore, sni string) (certPEM, keyPEM string, err error) {
	certPEM, err = st.GetSetting(ctx, keyHysteria2Cert)
	if err != nil {
		return "", "", err
	}
	keyPEM, err = st.GetSetting(ctx, keyHysteria2Key)
	if err != nil {
		return "", "", err
	}
	if certPEM != "" && keyPEM != "" {
		return certPEM, keyPEM, nil
	}

	certPEM, keyPEM, err = generateSelfSigned(sni)
	if err != nil {
		return "", "", err
	}
	if err = st.SetSetting(ctx, keyHysteria2Cert, certPEM); err != nil {
		return "", "", err
	}
	if err = st.SetSetting(ctx, keyHysteria2Key, keyPEM); err != nil {
		return "", "", err
	}
	return certPEM, keyPEM, nil
}

func generateSelfSigned(sni string) (certPEM, keyPEM string, err error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return "", "", err
	}
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: sni},
		DNSNames:              []string{sni},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return "", "", err
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return "", "", err
	}
	certPEM = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	keyPEM = string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}))
	return certPEM, keyPEM, nil
}

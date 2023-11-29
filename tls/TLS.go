package tls

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"time"
)


func GenerateTLSCert(org string) (*tls.Certificate, error) {
	privKey, genPrivKeyErr := generatePrivateKey()
	if genPrivKeyErr != nil { return nil, genPrivKeyErr }

	certBytes, genSelfSignedErr := createSelfSignedCert(org, privKey)
	if genSelfSignedErr != nil { return nil, genSelfSignedErr }

	return &tls.Certificate{
		Certificate: [][]byte{ certBytes },
		PrivateKey:  privKey,
	}, nil
}

func createSelfSignedCert(org string, privKey *ecdsa.PrivateKey) ([]byte, error) {
	template, genCertErr := generateCertTemplate(org)
	if genCertErr != nil { return nil, genCertErr }

	certBytes, certErr := x509.CreateCertificate(rand.Reader, template, template, &privKey.PublicKey, privKey)
	if certErr != nil { return nil, certErr }

	return certBytes, nil
}

func generateCertTemplate(org string) (*x509.Certificate, error) {
	notBefore := time.Now()
	notAfter := notBefore.Add(365 * 24 * time.Hour)

	serialNumber, randErr := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if randErr != nil { return nil, randErr }

	return &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{ Organization: []string{ org }},
		NotBefore: notBefore,
		NotAfter: notAfter,
		KeyUsage: x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{ x509.ExtKeyUsageServerAuth },
		BasicConstraintsValid: true,
	}, nil
}

func generatePrivateKey() (*ecdsa.PrivateKey, error) {
	privKey, genErr := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if genErr != nil { return nil, genErr }

	return privKey, nil
}
package certmanager

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"time"

	"github.com/rakutentech/shibuya/shibuya/config"
)

func CertTemplate(commonName string, ipAddr net.IP) (*x509.Certificate, error) {
	// generate a random serial number (a real cert authority would have some logic behind this)
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, errors.New("failed to generate serial number: " + err.Error())
	}
	tmpl := x509.Certificate{
		SerialNumber:          serialNumber,
		Subject:               pkix.Name{CommonName: "shibuya-coordinator"},
		SignatureAlgorithm:    x509.SHA256WithRSA,
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour * 24 * 2), // valid for two hours
		BasicConstraintsValid: true,
	}
	if ipAddr != nil {
		tmpl.IPAddresses = []net.IP{ipAddr}
	}
	return &tmpl, nil
}

// CreateCert invokes x509.CreateCertificate and returns it in the x509.Certificate format
func CreateCert(template, parent *x509.Certificate, pub interface{}, parentPriv *rsa.PrivateKey) (
	certPEM []byte, err error) {

	certDER, err := x509.CreateCertificate(rand.Reader, template, parent, pub, parentPriv)
	if err != nil {
		return
	}
	// PEM encode the certificate (this is a standard TLS encoding)
	b := pem.Block{Type: "CERTIFICATE", Bytes: certDER}
	certPEM = pem.EncodeToMemory(&b)
	return
}

func GenCertAndKey(caPair *config.CAPair, projectID int64, ip string) ([]byte, []byte, error) {
	ipAddr := net.ParseIP(ip)
	if ipAddr == nil && ip != "" {
		return nil, nil, fmt.Errorf("invalid IP address: %s", ip)
	}
	servKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}
	// create a template for the server
	servCertTmpl, err := CertTemplate("", ipAddr)
	if err != nil {
		return nil, nil, err
	}
	servCertTmpl.KeyUsage = x509.KeyUsageDigitalSignature
	servCertTmpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}

	// create a certificate which wraps the server's public key, sign it with the root private key
	servCertPEM, err := CreateCert(servCertTmpl, caPair.Cert, &servKey.PublicKey, caPair.PrivateKey)
	if err != nil {
		return nil, nil, err
	}

	// provide the private key and the cert
	servKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(servKey),
	})
	return servCertPEM, servKeyPEM, nil
}

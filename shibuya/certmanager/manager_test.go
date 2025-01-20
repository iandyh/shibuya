package certmanager_test

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/rakutentech/shibuya/shibuya/certmanager"
	"github.com/rakutentech/shibuya/shibuya/config"
	"github.com/stretchr/testify/assert"
)

// This will assume the CA is created
func loadCA() (*config.CAPair, error) {
	_, currentFile, _, _ := runtime.Caller(0)
	caFolder := filepath.Join(filepath.Dir(filepath.Dir(currentFile)), "ca")
	caCertPath := filepath.Join(caFolder, "shibuya-rootca.crt")
	caKeyPath := filepath.Join(caFolder, "shibuya-rootca.key")
	return config.LoadCaCert(caCertPath, caKeyPath), nil
}

func pemDecode(orig []byte) []byte {
	blocks, _ := pem.Decode(orig)
	return blocks.Bytes
}

func TestGenCertAndKey(t *testing.T) {
	projectID := int64(1)
	caPair, err := loadCA()
	assert.Nil(t, err)
	cert, key, err := certmanager.GenCertAndKey(caPair, projectID, "127.0.0.1")
	assert.Nil(t, err)
	assert.Greater(t, len(cert), 0)
	assert.Greater(t, len(key), 0)
	assert.Nil(t, err)

	os.WriteFile("test-cert.pem", cert, 0644)
	os.WriteFile("test-key.pem", key, 0644)
	handler := http.NewServeMux()
	handler.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello, HTTPS!")
	})
	keyPair, err := tls.LoadX509KeyPair("test-cert.pem", "test-key.pem")
	assert.Nil(t, err)
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{keyPair},
	}
	server := httptest.NewUnstartedServer(handler)
	server.TLS = tlsConfig
	server.StartTLS()

	pool := x509.NewCertPool()
	pool.AddCert(caPair.Cert)
	client := &http.Client{
		Timeout: 3 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: pool,
			},
		},
	}
	resp, err := client.Get(server.URL)
	assert.Nil(t, err)
	defer resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)
}

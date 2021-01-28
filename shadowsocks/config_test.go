// Copyright 2021 The Outline Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package shadowsocks

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"reflect"
	"testing"
	"time"
)

const (
	redirectURL    = "https://127.0.0.1/200/"
	examplePemCert = `-----BEGIN CERTIFICATE-----
MIIG1TCCBb2gAwIBAgIQD74IsIVNBXOKsMzhya/uyTANBgkqhkiG9w0BAQsFADBP
MQswCQYDVQQGEwJVUzEVMBMGA1UEChMMRGlnaUNlcnQgSW5jMSkwJwYDVQQDEyBE
aWdpQ2VydCBUTFMgUlNBIFNIQTI1NiAyMDIwIENBMTAeFw0yMDExMjQwMDAwMDBa
Fw0yMTEyMjUyMzU5NTlaMIGQMQswCQYDVQQGEwJVUzETMBEGA1UECBMKQ2FsaWZv
cm5pYTEUMBIGA1UEBxMLTG9zIEFuZ2VsZXMxPDA6BgNVBAoTM0ludGVybmV0IENv
cnBvcmF0aW9uIGZvciBBc3NpZ25lZCBOYW1lcyBhbmQgTnVtYmVyczEYMBYGA1UE
AxMPd3d3LmV4YW1wbGUub3JnMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKC
AQEAuvzuzMoKCP8Okx2zvgucA5YinrFPEK5RQP1TX7PEYUAoBO6i5hIAsIKFmFxt
W2sghERilU5rdnxQcF3fEx3sY4OtY6VSBPLPhLrbKozHLrQ8ZN/rYTb+hgNUeT7N
A1mP78IEkxAj4qG5tli4Jq41aCbUlCt7equGXokImhC+UY5IpQEZS0tKD4vu2ksZ
04Qetp0k8jWdAvMA27W3EwgHHNeVGWbJPC0Dn7RqPw13r7hFyS5TpleywjdY1nB7
ad6kcZXZbEcaFZ7ZuerA6RkPGE+PsnZRb1oFJkYoXimsuvkVFhWeHQXCGC1cuDWS
rM3cpQvOzKH2vS7d15+zGls4IwIDAQABo4IDaTCCA2UwHwYDVR0jBBgwFoAUt2ui
6qiqhIx56rTaD5iyxZV2ufQwHQYDVR0OBBYEFCYa+OSxsHKEztqBBtInmPvtOj0X
MIGBBgNVHREEejB4gg93d3cuZXhhbXBsZS5vcmeCC2V4YW1wbGUuY29tggtleGFt
cGxlLmVkdYILZXhhbXBsZS5uZXSCC2V4YW1wbGUub3Jngg93d3cuZXhhbXBsZS5j
b22CD3d3dy5leGFtcGxlLmVkdYIPd3d3LmV4YW1wbGUubmV0MA4GA1UdDwEB/wQE
AwIFoDAdBgNVHSUEFjAUBggrBgEFBQcDAQYIKwYBBQUHAwIwgYsGA1UdHwSBgzCB
gDA+oDygOoY4aHR0cDovL2NybDMuZGlnaWNlcnQuY29tL0RpZ2lDZXJ0VExTUlNB
U0hBMjU2MjAyMENBMS5jcmwwPqA8oDqGOGh0dHA6Ly9jcmw0LmRpZ2ljZXJ0LmNv
bS9EaWdpQ2VydFRMU1JTQVNIQTI1NjIwMjBDQTEuY3JsMEwGA1UdIARFMEMwNwYJ
YIZIAYb9bAEBMCowKAYIKwYBBQUHAgEWHGh0dHBzOi8vd3d3LmRpZ2ljZXJ0LmNv
bS9DUFMwCAYGZ4EMAQICMH0GCCsGAQUFBwEBBHEwbzAkBggrBgEFBQcwAYYYaHR0
cDovL29jc3AuZGlnaWNlcnQuY29tMEcGCCsGAQUFBzAChjtodHRwOi8vY2FjZXJ0
cy5kaWdpY2VydC5jb20vRGlnaUNlcnRUTFNSU0FTSEEyNTYyMDIwQ0ExLmNydDAM
BgNVHRMBAf8EAjAAMIIBBQYKKwYBBAHWeQIEAgSB9gSB8wDxAHcA9lyUL9F3MCIU
VBgIMJRWjuNNExkzv98MLyALzE7xZOMAAAF1+73YbgAABAMASDBGAiEApGuo0EOk
8QcyLe2cOX136HPBn+0iSgDFvprJtbYS3LECIQCN6F+Kx1LNDaEj1bW729tiE4gi
1nDsg14/yayUTIxYOgB2AFzcQ5L+5qtFRLFemtRW5hA3+9X6R9yhc5SyXub2xw7K
AAABdfu92M0AAAQDAEcwRQIgaqwR+gUJEv+bjokw3w4FbsqOWczttcIKPDM0qLAz
2qwCIQDa2FxRbWQKpqo9izUgEzpql092uWfLvvzMpFdntD8bvTANBgkqhkiG9w0B
AQsFAAOCAQEApyoQMFy4a3ob+GY49umgCtUTgoL4ZYlXpbjrEykdhGzs++MFEdce
MV4O4sAA5W0GSL49VW+6txE1turEz4TxMEy7M54RFyvJ0hlLLNCtXxcjhOHfF6I7
qH9pKXxIpmFfJj914jtbozazHM3jBFcwH/zJ+kuOSIBYJ5yix8Mm3BcC+uZs6oEB
XJKP0xgIF3B6wqNLbDr648/2/n7JVuWlThsUT6mYnXmxHsOrsQ0VhalGtuXCWOha
/sgUKGiQxrjIlH/hD4n6p9YJN6FitwAntb7xsV5FKAazVBXmw8isggHOhuIr4Xrk
vUzLnF7QYsJhvYtaYrZ2MLxGD+NFI8BkXw==
-----END CERTIFICATE-----`
	exampleCertFingerprintBase64 = "IA3K+nZ8hFDs5kSHnAYqDN9SJA/gW7frKEYRw67z7C4="
)

var proxies = []Config{
	{"0", "ssconf.test", 123, "passw0rd", "chacha20-ietf-poly1305", "ssconf-test-1", "plugin", "opts"},
	{"1", "ssconf-ii.test", 456, "dr0wssap", "chacha20-ietf-poly1305", "ssconf-test-2", "", ""},
}

func TestFetchConfig(t *testing.T) {
	cert, err := makeTLSCertificate()
	if err != nil {
		t.Fatalf("Failed to generate TLS certificate: %v", err)
	}

	certFingerprint := computeCertificateFingerprint(cert.Certificate[0])
	server := makeOnlineConfigServer(cert)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start online config server: %v", err)
	}
	serverAddr := listener.Addr()
	go server.ServeTLS(listener, "", "")
	defer server.Close()

	t.Run("Success", func(t *testing.T) {
		req := FetchConfigRequest{
			fmt.Sprintf("https://%s/200", serverAddr), "GET", certFingerprint}
		res, err := FetchConfig(req)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if res.HTTPStatusCode != 200 {
			t.Errorf("Expected 200 HTTP status code, got %d", res.HTTPStatusCode)
		}
		if res.RedirectURL != "" {
			t.Errorf("Unexpected redirect URL: %s", res.RedirectURL)
		}
		if !reflect.DeepEqual(proxies, res.Config.Proxies) {
			t.Errorf("Proxy configurations don't match. Want %v, have %v",
				proxies, res.Config.Proxies)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		req := FetchConfigRequest{
			fmt.Sprintf("https://%s/404", serverAddr), "GET", certFingerprint}
		res, err := FetchConfig(req)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if res.HTTPStatusCode != 404 {
			t.Errorf("Expected 404 HTTP status code, got %d", res.HTTPStatusCode)
		}
		if res.RedirectURL != "" {
			t.Errorf("Unexpected redirect URL: %s", res.RedirectURL)
		}
		if len(res.Config.Proxies) > 0 {
			t.Errorf("Expected empty proxy configurations, got: %v",
				res.Config.Proxies)
		}
	})

	t.Run("Redirect", func(t *testing.T) {
		req := FetchConfigRequest{
			fmt.Sprintf("https://%s/301", serverAddr), "GET", certFingerprint}
		res, err := FetchConfig(req)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if res.HTTPStatusCode != 301 {
			t.Errorf("Expected 301 HTTP status code, got %d", res.HTTPStatusCode)
		}
		if res.RedirectURL != redirectURL {
			t.Errorf("Expected redirect URL %s , got %s", redirectURL, res.RedirectURL)
		}
		if len(res.Config.Proxies) > 0 {
			t.Errorf("Expected empty proxy configurations, got: %v", res.Config.Proxies)
		}
	})

	t.Run("WrongCertificateFingerprint", func(t *testing.T) {
		wrongCertFp := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
		req := FetchConfigRequest{
			fmt.Sprintf("https://%s/success", serverAddr), "GET", wrongCertFp}
		_, err := FetchConfig(req)
		if err == nil {
			t.Errorf("Expected TLS certificate validation error")
		}
		var certErr x509.CertificateInvalidError
		if !errors.As(err, &certErr) {
			t.Errorf("Expected invalid certificate error, got %v",
				reflect.TypeOf(err))
		}
	})

	t.Run("MissingCertificateFingerprint", func(t *testing.T) {
		req := FetchConfigRequest{
			fmt.Sprintf("https://%s/success", serverAddr), "GET", nil}
		_, err := FetchConfig(req)
		if err == nil {
			t.Errorf("Expected certificate validation error")
		}
		var authErr x509.UnknownAuthorityError
		if !errors.As(err, &authErr) {
			t.Errorf("Expected unknown authority error, got %v",
				reflect.TypeOf(err))
		}
	})

	t.Run("Method", func(t *testing.T) {
		req := FetchConfigRequest{
			fmt.Sprintf("https://%s/200-post", serverAddr), "POST", certFingerprint}
		res, err := FetchConfig(req)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if res.HTTPStatusCode != 200 {
			t.Errorf("Expected 200 HTTP status code, got %d", res.HTTPStatusCode)
		}
		if !reflect.DeepEqual(proxies, res.Config.Proxies) {
			t.Errorf("Proxy configurations don't match. Want %v, have %v",
				proxies, res.Config.Proxies)
		}
	})

	t.Run("NonHTTPSURL", func(t *testing.T) {
		req := FetchConfigRequest{
			fmt.Sprintf("http://%s/success", serverAddr), "GET", certFingerprint}
		_, err := FetchConfig(req)
		if err == nil {
			t.Fatalf("Expected error for non-HTTPs URL")
		}
	})

	t.Run("SyntaxError", func(t *testing.T) {
		req := FetchConfigRequest{
			fmt.Sprintf("https://%s/200-invalid-syntax", serverAddr), "GET", certFingerprint}
		_, err := FetchConfig(req)
		if err == nil {
			t.Errorf("Expected JSON syntax error")
		}
		var jsonErr *json.SyntaxError
		if !errors.As(err, &jsonErr) {
			t.Errorf("Expected JSON syntax error, got %v", reflect.TypeOf(err))
		}
	})

	t.Run("DecodingError", func(t *testing.T) {
		req := FetchConfigRequest{
			fmt.Sprintf("https://%s/200-invalid-type", serverAddr), "GET", certFingerprint}
		_, err := FetchConfig(req)
		if err == nil {
			t.Errorf("Expected JSON decoding error")
		}
		var jsonErr *json.UnmarshalTypeError
		if !errors.As(err, &jsonErr) {
			t.Errorf("Expected JSON decoding error, got %v", reflect.TypeOf(err))
		}
	})
}

// HTTP handler for a fake online config server.
type onlineConfigHandler struct{}

func (h onlineConfigHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path == "/200" {
		h.sendOnlineConfig(w)
	} else if req.URL.Path == "/301" {
		w.Header().Add("Location", redirectURL)
		h.sendResponse(w, 301, []byte{})
	} else if req.URL.Path == "/200-post" && req.Method == "POST" {
		h.sendOnlineConfig(w)
	} else if req.URL.Path == "/200-invalid-syntax" {
		h.sendResponse(w, 200, []byte("{invalid SIP008 JSON}"))
	} else if req.URL.Path == "/200-invalid-type" {
		h.sendResponse(w, 200, []byte(`{"version": 1, "servers": [{"server": 123}]}`))
	} else {
		h.sendResponse(w, 404, []byte("Not Found"))
	}
}

func (h onlineConfigHandler) sendOnlineConfig(w http.ResponseWriter) {
	res := OnlineConfig{proxies, 1}
	data, _ := json.Marshal(res)
	h.sendResponse(w, 200, data)
}

func (onlineConfigHandler) sendResponse(w http.ResponseWriter, code int, data []byte) {
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(data)
}

// Returns a SIP008 online config HTTPs server with TLS certificate cert.
func makeOnlineConfigServer(cert tls.Certificate) http.Server {
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}
	return http.Server{
		TLSConfig: tlsConfig,
		Handler:   onlineConfigHandler{},
	}
}

// Generates a self-signed TLS certificate for localhost.
func makeTLSCertificate() (tls.Certificate, error) {
	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: big.NewInt(now.Unix()),
		Subject: pkix.Name{
			Organization: []string{"online config"},
		},
		IPAddresses:           []net.IP{net.IPv4(127, 0, 0, 1)}, // Valid for localhost
		NotBefore:             now,
		NotAfter:              now.AddDate(0, 0, 1), // Valid for one day
		BasicConstraintsValid: true,
		IsCA:                  true, // Self-signed
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		KeyUsage: x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature |
			x509.KeyUsageCertSign,
	}

	key, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return tls.Certificate{}, err
	}

	derCert, err := x509.CreateCertificate(rand.Reader, template, template,
		key.Public(), key)
	if err != nil {
		return tls.Certificate{}, err
	}

	var cert tls.Certificate
	cert.Certificate = append(cert.Certificate, derCert)
	cert.PrivateKey = key
	return cert, nil
}

func TestComputeCertificateFingerprint(t *testing.T) {
	pemCertData := []byte(examplePemCert)
	block, _ := pem.Decode(pemCertData)
	if block == nil || block.Type != "CERTIFICATE" {
		t.Fatalf("Failed to decode certificate PEM block")
	}
	expectedCertFp, err := base64.StdEncoding.DecodeString(exampleCertFingerprintBase64)
	if err != nil {
		t.Fatalf("Failed to decode certificate fingerprint: %v", err)
	}
	actualCertFp := computeCertificateFingerprint(block.Bytes)
	if !bytes.Equal(actualCertFp, expectedCertFp) {
		t.Errorf("Certificate fingerprints don't match. Want %v, got %v",
			expectedCertFp, actualCertFp)
	}
}

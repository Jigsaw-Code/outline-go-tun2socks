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
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"reflect"
	"testing"
	"time"
)

const redirectURL = "https://127.0.0.1/200/"

var proxies = []ProxyConfig{
	{"0", "ssconf.test", 123, "passw0rd", "chacha20-ietf-poly1305", "ssconf-test-1", "plugin", "opts"},
	{"1", "ssconf-ii.test", 456, "dr0wssap", "chacha20-ietf-poly1305", "ssconf-test-2", "", ""},
}

func TestFetchOnlineConfig(t *testing.T) {
	cert, err := makeTLSCertificate()
	if err != nil {
		t.Fatalf("Failed to generate TLS certificate: %v", err)
	}

	certFingerprintBytes := sha256.Sum256(cert.Certificate[0])
	certFingerprint := certFingerprintBytes[:]
	server := makeOnlineConfigServer(cert)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start online config server: %v", err)
	}
	serverAddr := listener.Addr()
	go server.ServeTLS(listener, "", "")
	defer server.Close()

	t.Run("Success", func(t *testing.T) {
		req := OnlineConfigRequest{
			fmt.Sprintf("https://%s/200", serverAddr), "GET", certFingerprint}
		res, err := FetchOnlineConfig(req)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if res.HTTPStatusCode != 200 {
			t.Errorf("Expected 200 HTTP status code, got %d", res.HTTPStatusCode)
		}
		if res.RedirectURL != "" {
			t.Errorf("Unexpected redirect URL: %s", res.RedirectURL)
		}
		if !reflect.DeepEqual(proxies, res.OnlineConfig.Proxies) {
			t.Errorf("Proxy configurations don't match. Want %v, have %v",
				proxies, res.OnlineConfig.Proxies)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		req := OnlineConfigRequest{
			fmt.Sprintf("https://%s/404", serverAddr), "GET", certFingerprint}
		res, err := FetchOnlineConfig(req)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if res.HTTPStatusCode != 404 {
			t.Errorf("Expected 404 HTTP status code, got %d", res.HTTPStatusCode)
		}
		if res.RedirectURL != "" {
			t.Errorf("Unexpected redirect URL: %s", res.RedirectURL)
		}
		if len(res.OnlineConfig.Proxies) > 0 {
			t.Errorf("Expected empty proxy configurations, got: %v",
				res.OnlineConfig.Proxies)
		}
	})

	t.Run("Redirect", func(t *testing.T) {
		req := OnlineConfigRequest{
			fmt.Sprintf("https://%s/301", serverAddr), "GET", certFingerprint}
		res, err := FetchOnlineConfig(req)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if res.HTTPStatusCode != 301 {
			t.Errorf("Expected 301 HTTP status code, got %d", res.HTTPStatusCode)
		}
		if res.RedirectURL != redirectURL {
			t.Errorf("Expected redirect URL %s , got %s", redirectURL, res.RedirectURL)
		}
		if len(res.OnlineConfig.Proxies) > 0 {
			t.Errorf("Expected empty proxy configurations, got: %v", res.OnlineConfig.Proxies)
		}
	})

	t.Run("WrongCertificateFingerprint", func(t *testing.T) {
		wrongCertFp := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
		req := OnlineConfigRequest{
			fmt.Sprintf("https://%s/success", serverAddr), "GET", wrongCertFp}
		_, err := FetchOnlineConfig(req)
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
		req := OnlineConfigRequest{
			fmt.Sprintf("https://%s/success", serverAddr), "GET", nil}
		_, err := FetchOnlineConfig(req)
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
		req := OnlineConfigRequest{
			fmt.Sprintf("https://%s/200-post", serverAddr), "POST", certFingerprint}
		res, err := FetchOnlineConfig(req)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if res.HTTPStatusCode != 200 {
			t.Errorf("Expected 200 HTTP status code, got %d", res.HTTPStatusCode)
		}
		if !reflect.DeepEqual(proxies, res.OnlineConfig.Proxies) {
			t.Errorf("Proxy configurations don't match. Want %v, have %v",
				proxies, res.OnlineConfig.Proxies)
		}
	})

	t.Run("NonHTTPSURL", func(t *testing.T) {
		req := OnlineConfigRequest{
			fmt.Sprintf("http://%s/success", serverAddr), "GET", certFingerprint}
		_, err := FetchOnlineConfig(req)
		if err == nil {
			t.Fatalf("Expected error for non-HTTPs URL")
		}
	})

	t.Run("SyntaxError", func(t *testing.T) {
		req := OnlineConfigRequest{
			fmt.Sprintf("https://%s/200-invalid-syntax", serverAddr), "GET", certFingerprint}
		_, err := FetchOnlineConfig(req)
		if err == nil {
			t.Errorf("Expected JSON syntax error")
		}
		var jsonErr *json.SyntaxError
		if !errors.As(err, &jsonErr) {
			t.Errorf("Expected JSON syntax error, got %v", reflect.TypeOf(err))
		}
	})

	t.Run("DecodingError", func(t *testing.T) {
		req := OnlineConfigRequest{
			fmt.Sprintf("https://%s/200-invalid-type", serverAddr), "GET", certFingerprint}
		_, err := FetchOnlineConfig(req)
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

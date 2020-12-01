// Copyright 2020 The Outline Authors
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

package doh

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"sync"
)

// CertificateLoader interface for requesting ClientAuth instances.
type CertificateLoader interface {
	// Request a ClientAuth instance (blocking).
	// returns nil (no authentication) or a ClientAuth instance.
	LoadClientCertificate() ClientAuth
}

// ClientAuth interface for providing TLS certificates and signatures.
type ClientAuth interface {
	// GetClientCertificate returns the client certificate (if any).
	// It does not block or cause certificates to load.
	// Returns a DER encoded X.509 client certificate.
	GetClientCertificate() []byte
	// GetIntermediateCertificate returns the chaining certificate (if any).
	// It does not block or cause certificates to load.
	// Returns a DER encoded X.509 certificate.
	GetIntermediateCertificate() []byte
	// Request a signature on a digest.
	Sign(digest []byte) []byte
}

// clientAuthWrapper manages certificate loading and usage during TLS handshakes.
// Implements crypto.Signer.
type clientAuthWrapper struct {
	sync.Mutex
	loadCertificateOnce sync.Once
	loader              CertificateLoader

	certificate tls.Certificate
	signer      ClientAuth
}

func (ca *clientAuthWrapper) loadClientCertificate() {
	// Ensure that any previous certificate is cleared regardless of success.
	ca.certificate = tls.Certificate{}
	// If no loader was provided then we can't load a certificate.
	if ca.loader == nil {
		return
	}
	signer := ca.loader.LoadClientCertificate()
	if signer == nil {
		return
	}
	cert := signer.GetClientCertificate()
	if cert == nil {
		return
	}
	intermediate := signer.GetIntermediateCertificate()
	chain := make([][]byte, 0, 2)
	chain = append(chain, cert)
	if intermediate != nil {
		chain = append(chain, intermediate)
	}
	leaf, err := x509.ParseCertificate(cert)
	if err != nil {
		return
	}
	_, isECDSA := leaf.PublicKey.(*ecdsa.PublicKey)
	if !isECDSA {
		// RSA-PSS and RSA-SSA both need explicit signature generation support.
		// Fail here rather than during signing.
		return
	}
	ca.certificate = tls.Certificate{
		Certificate: chain,
		PrivateKey:  ca,
		Leaf:        leaf,
	}
	ca.signer = signer
}

func (ca *clientAuthWrapper) finalizeClientAuth() {
	// Attempt to set signer on the first call.
	// Subsequent callers will block until this completes,
	// ensuring ca.signer and ca.certificate are both safe to access.
	ca.Lock()
	defer ca.Unlock()
	ca.loadCertificateOnce.Do(ca.loadClientCertificate)
}

// Fetch the client certificate from the ClientAuth provider.
// Implements tls.Config GetClientCertificate().
func (ca *clientAuthWrapper) GetClientCertificate(
	info *tls.CertificateRequestInfo) (*tls.Certificate, error) {
	ca.finalizeClientAuth()
	return &ca.certificate, nil
}

// Public returns the public key for the client certificate.
func (ca *clientAuthWrapper) Public() crypto.PublicKey {
	ca.finalizeClientAuth()
	cert := ca.certificate
	if cert.Leaf == nil {
		return nil
	}
	return cert.Leaf.PublicKey
}

// Sign a digest.
func (ca *clientAuthWrapper) Sign(rand io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	ca.finalizeClientAuth()
	if ca.signer == nil {
		return nil, errors.New("no client certificate")
	}
	signature := ca.signer.Sign(digest)
	if signature == nil {
		return nil, errors.New("failed to create signature")
	}
	return signature, nil
}

func newClientAuthWrapper(loader CertificateLoader) clientAuthWrapper {
	return clientAuthWrapper{
		loader: loader,
	}
}

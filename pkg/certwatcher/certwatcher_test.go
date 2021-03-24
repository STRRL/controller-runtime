/*
Copyright 2021 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package certwatcher_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/certwatcher"
)

var _ = Describe("CertWatcher", func() {
	var _ = Describe("certwatcher New", func() {
		It("should errors without cert/key", func() {
			_, err := certwatcher.New("", "")
			Expect(err).ToNot(BeNil())
		})
	})

	var _ = Describe("certwatcher Start", func() {
		var (
			ctx       context.Context
			ctxCancel context.CancelFunc
			watcher   *certwatcher.CertWatcher
		)

		BeforeEach(func() {
			ctx, ctxCancel = context.WithCancel(context.Background())

			err := writeCerts(certPath, keyPath, "127.0.0.1")
			Expect(err).To(BeNil())

			Eventually(func() error {
				for _, file := range []string{certPath, keyPath} {
					_, err := os.ReadFile(file)
					if err != nil {
						return err
					}
					continue
				}

				return nil
			}).Should(Succeed())

			watcher, err = certwatcher.New(certPath, keyPath)
			Expect(err).To(BeNil())
		})

		startWatcher := func() (done <-chan struct{}) {
			doneCh := make(chan struct{})
			go func() {
				defer GinkgoRecover()
				defer close(doneCh)
				Expect(watcher.Start(ctx)).To(Succeed())
			}()
			// wait till we read first cert
			Eventually(func() error {
				err := watcher.ReadCertificate()
				return err
			}).Should(Succeed())
			return doneCh
		}

		It("should read the initial cert/key", func() {
			doneCh := startWatcher()

			ctxCancel()
			Eventually(doneCh, "4s").Should(BeClosed())
		})

		It("should reload currentCert when changed", func() {
			doneCh := startWatcher()

			firstcert, _ := watcher.GetCertificate(nil)

			err := writeCerts(certPath, keyPath, "192.168.0.1")
			Expect(err).To(BeNil())

			Eventually(func() bool {
				secondcert, _ := watcher.GetCertificate(nil)
				first := firstcert.PrivateKey.(*rsa.PrivateKey)
				return first.Equal(secondcert.PrivateKey)
			}).ShouldNot(BeTrue())

			ctxCancel()
			Eventually(doneCh, "4s").Should(BeClosed())
		})
	})
})

func writeCerts(certPath, keyPath, ip string) error {
	var priv interface{}
	var err error
	priv, err = rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}

	keyUsage := x509.KeyUsageDigitalSignature
	if _, isRSA := priv.(*rsa.PrivateKey); isRSA {
		keyUsage |= x509.KeyUsageKeyEncipherment
	}

	var notBefore time.Time
	notBefore = time.Now()

	notAfter := notBefore.Add(1 * time.Hour)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return err
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Kubernetes"},
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage:              keyUsage,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	template.IPAddresses = append(template.IPAddresses, net.ParseIP(ip))

	privkey := priv.(*rsa.PrivateKey)

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privkey.PublicKey, priv)
	if err != nil {
		return err
	}

	certOut, err := os.Create(certPath)
	if err != nil {
		return err
	}
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return err
	}
	if err := certOut.Close(); err != nil {
		return err
	}

	keyOut, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return err
	}
	if err := pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}); err != nil {
		return err
	}
	if err := keyOut.Close(); err != nil {
		return err
	}
	return nil
}
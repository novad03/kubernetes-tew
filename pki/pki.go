package pki

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"net"
	"os"
	"time"

	"github.com/darxkies/k8s-tew/utils"
	log "github.com/sirupsen/logrus"
)

func GenerateEncryptionConfig() (string, error) {
	buffer := make([]byte, 32)

	_, error := rand.Read(buffer)

	if error != nil {
		return "", error
	}

	return base64.StdEncoding.EncodeToString(buffer), nil
}

func newBigInt() (*big.Int, error) {
	return rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 160))
}

func bigIntHash() ([]byte, error) {
	number, error := newBigInt()
	if error != nil {
		return nil, error
	}

	hash := sha1.New()

	hash.Write(number.Bytes())

	return hash.Sum(nil), nil
}

type CertificateAndPrivateKey struct {
	CertificateFilename string
	PrivateKeyFilename  string
	Certificate         *x509.Certificate
	PrivateKey          *rsa.PrivateKey
}

func loadPEMBlock(filename string) (*pem.Block, error) {
	file, error := os.Open(filename)
	if error != nil {
		return nil, error
	}

	defer file.Close()

	raw, error := ioutil.ReadAll(file)
	if error != nil {
		return nil, error
	}

	block, _ := pem.Decode(raw)

	return block, nil
}

func LoadCertificateAndPrivateKey(certificateFilename, privateKeyFilename string) (*CertificateAndPrivateKey, error) {
	result := &CertificateAndPrivateKey{CertificateFilename: certificateFilename, PrivateKeyFilename: privateKeyFilename}

	block, error := loadPEMBlock(certificateFilename)
	if error != nil {
		return nil, error
	}

	if block == nil || block.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("wrong certificate format in '%s'", certificateFilename)
	}

	result.Certificate, error = x509.ParseCertificate(block.Bytes)
	if error != nil {
		return nil, error
	}

	block, error = loadPEMBlock(privateKeyFilename)
	if error != nil {
		return nil, error
	}

	if block == nil || block.Type != "RSA PRIVATE KEY" {
		return nil, fmt.Errorf("wrong private key format in '%s'", privateKeyFilename)
	}

	result.PrivateKey, error = x509.ParsePKCS1PrivateKey(block.Bytes)
	if error != nil {
		return nil, error
	}

	return result, nil
}

func newTemplate(validityPeriod int, commonName, organization string) (*x509.Certificate, error) {
	serialNumber, error := newBigInt()
	if error != nil {
		return nil, error
	}

	subjectKeyId, error := bigIntHash()
	if error != nil {
		return nil, error
	}

	now := time.Now()

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		SubjectKeyId: subjectKeyId,
		Subject: pkix.Name{
			CommonName:   commonName,
			Organization: []string{organization},
		},
		NotBefore: now.Add(-5 * time.Minute),
		NotAfter:  now.AddDate(validityPeriod, 0, 0),
	}

	return template, nil
}

func createAndSaveCertificate(signer *CertificateAndPrivateKey, template *x509.Certificate, rsaSize int, certificateFilename, privateKeyFilename string) error {
	var error error

	privateKey, error := rsa.GenerateKey(rand.Reader, rsaSize)
	if error != nil {
		return error
	}

	if signer == nil {
		signer = &CertificateAndPrivateKey{Certificate: template, PrivateKey: privateKey}
	}

	certificateData, error := x509.CreateCertificate(rand.Reader, template, signer.Certificate, &privateKey.PublicKey, signer.PrivateKey)
	if error != nil {
		return error
	}

	certificatePEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificateData})

	if error := ioutil.WriteFile(certificateFilename, certificatePEM, 0644); error != nil {
		return error
	}

	privateKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})

	if error := ioutil.WriteFile(privateKeyFilename, privateKeyPEM, 0644); error != nil {
		return error
	}

	log.WithFields(log.Fields{"filename": certificateFilename}).Info("generated")
	log.WithFields(log.Fields{"filename": privateKeyFilename}).Info("generated")

	return nil
}

func GenerateCA(rsaSize int, validityPeriod int, commonName, organization, certificateFilename, privateKeyFilename string) error {
	if utils.FileExists(certificateFilename) && utils.FileExists(privateKeyFilename) {
		log.WithFields(log.Fields{"filename": certificateFilename}).Info("skipped")
		log.WithFields(log.Fields{"filename": privateKeyFilename}).Info("skipped")

		return nil
	}

	template, error := newTemplate(validityPeriod, commonName, organization)
	if error != nil {
		return error
	}

	template.KeyUsage = x509.KeyUsageCertSign | x509.KeyUsageCRLSign
	template.BasicConstraintsValid = true
	template.IsCA = true
	template.MaxPathLen = 2

	return createAndSaveCertificate(nil, template, rsaSize, certificateFilename, privateKeyFilename)
}

func GenerateClient(signer *CertificateAndPrivateKey, rsaSize int, validityPeriod int, commonName, organization string, dnsNames []string, ipAddresses []string, certificateFilename, privateKeyFilename string, force bool) error {
	if utils.FileExists(certificateFilename) && utils.FileExists(privateKeyFilename) && !force {
		log.WithFields(log.Fields{"filename": certificateFilename}).Info("skipped")
		log.WithFields(log.Fields{"filename": privateKeyFilename}).Info("skipped")

		return nil
	}

	template, error := newTemplate(validityPeriod, commonName, organization)
	if error != nil {
		return error
	}

	template.KeyUsage = x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature
	template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}

	template.IPAddresses = []net.IP{}

	for _, ipString := range ipAddresses {
		ipAddress := net.ParseIP(ipString)

		if ipAddress == nil {
			return errors.New(fmt.Sprintf("'%s' is not a valid IP address", ipString))
		}

		template.IPAddresses = append(template.IPAddresses, ipAddress)
	}

	template.DNSNames = dnsNames

	return createAndSaveCertificate(signer, template, rsaSize, certificateFilename, privateKeyFilename)
}

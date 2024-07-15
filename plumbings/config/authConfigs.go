package config

import (
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"gitlab.com/benedictjohannes/b64uuid"
)

type GoogleAuthType struct {
	ClientId     string `yaml:"ClientId"`
	ClientSecret string `yaml:"ClientSecret"`
}
type MicrosoftAuthType struct {
	ClientId       string `yaml:"ClientId"`
	ClientCertPem  string `yaml:"ClientCertPem"`
	ClientKeyPem   string `yaml:"ClientKeyPem"`
	ClientKeyRsa   *rsa.PrivateKey
	ClientCertHash *string
	mu             sync.Mutex
	shouldRotateAt time.Time
	currentToken   *string
}

func (m *MicrosoftAuthType) Parse() (err error) {
	if m.ClientKeyRsa != nil && m.ClientCertHash != nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	pemBlock, _ := pem.Decode([]byte(m.ClientKeyPem))
	parsedKey, err := x509.ParsePKCS8PrivateKey(pemBlock.Bytes)
	if err != nil {
		return
	}
	var ok bool
	if m.ClientKeyRsa, ok = parsedKey.(*rsa.PrivateKey); !ok {
		return x509.ErrUnsupportedAlgorithm
	}
	pemBlock, _ = pem.Decode([]byte(m.ClientCertPem))
	_, err = x509.ParseCertificate(pemBlock.Bytes)
	if err != nil {
		return
	}
	hash := sha1.New()
	var hashB []byte
	hash.Write(pemBlock.Bytes)
	hashB = hash.Sum(nil)
	h := base64.RawStdEncoding.EncodeToString(hashB)
	m.ClientCertHash = &h
	return
}
func (m *MicrosoftAuthType) ReqUrl() string {
	return "https://login.microsoftonline.com/common/oauth2/v2.0/token"
}
func (m *MicrosoftAuthType) GetToken() (t string, err error) {
	err = m.Parse()
	if err != nil {
		return
	}
	now := time.Now()
	if m.shouldRotateAt.Before(now) || m.currentToken == nil {
		exp := now.Add(16 * time.Minute)
		m.shouldRotateAt = now.Add(15 * time.Minute)
		newToken := jwt.New(jwt.SigningMethodRS256)
		newToken.Header["typ"] = "JWT"
		newToken.Header["x5t"] = m.ClientCertHash
		newToken.Claims = jwt.MapClaims{
			"aud": m.ReqUrl(),
			"exp": exp.Unix(),
			"iss": m.ClientId,
			"jti": b64uuid.NewRandom(),
			"nbf": now.Add(-15 * time.Second).Unix(),
			"sub": m.ClientId,
			"iat": now.Unix(),
		}
		t, err = newToken.SignedString(m.ClientKeyRsa)
		m.currentToken = &t
	}
	return *m.currentToken, err
}

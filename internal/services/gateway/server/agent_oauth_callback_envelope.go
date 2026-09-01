package gateway

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"
)

const agentOAuthCallbackEnvelopeVersion = "v1"

var ErrAgentOAuthCallbackEnvelopeInvalid = errors.New("Agent OAuth callback envelope is invalid")

// AgentOAuthCallbackEnvelopeInput is the immutable binding authenticated by
// AES-GCM. Gateway provides only a Runtime public key, never private material.
type AgentOAuthCallbackEnvelopeInput struct {
	HandoffUUID, TransactionUUID, OwnerUserUUID string
	Issuer, RedirectURI                         string
	AuthorizationCode, AuthorizationCodeSHA256  string
	RuntimeKeyID, ExpiresAtRFC3339Millis        string
}

func SealAgentOAuthCallbackCode(input AgentOAuthCallbackEnvelopeInput, runtimePublicKeyPEM []byte) (string, error) {
	return sealAgentOAuthCallbackCode(rand.Reader, input, runtimePublicKeyPEM)
}

func sealAgentOAuthCallbackCode(random io.Reader, input AgentOAuthCallbackEnvelopeInput, runtimePublicKeyPEM []byte) (string, error) {
	if random == nil || !validAgentOAuthCallbackEnvelopeInput(input) {
		return "", ErrAgentOAuthCallbackEnvelopeInvalid
	}
	publicKey, err := parseAgentOAuthRuntimePublicKey(runtimePublicKeyPEM)
	if err != nil {
		return "", ErrAgentOAuthCallbackEnvelopeInvalid
	}
	dataKey := make([]byte, 32)
	defer clearAgentOAuthCallbackBytes(dataKey)
	if _, err = io.ReadFull(random, dataKey); err != nil {
		return "", fmt.Errorf("generate callback envelope data key: %w", err)
	}
	nonce := make([]byte, 12)
	if _, err = io.ReadFull(random, nonce); err != nil {
		return "", fmt.Errorf("generate callback envelope nonce: %w", err)
	}
	block, err := aes.NewCipher(dataKey)
	if err != nil {
		return "", fmt.Errorf("create callback envelope cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create callback envelope GCM: %w", err)
	}
	sealed := gcm.Seal(nil, nonce, []byte(input.AuthorizationCode), []byte(agentOAuthCallbackEnvelopeAAD(input)))
	ciphertext, tag := sealed[:len(sealed)-gcm.Overhead()], sealed[len(sealed)-gcm.Overhead():]
	wrapped, err := rsa.EncryptOAEP(sha256.New(), random, publicKey, dataKey, nil)
	if err != nil {
		return "", fmt.Errorf("wrap callback envelope data key: %w", err)
	}
	return strings.Join([]string{agentOAuthCallbackEnvelopeVersion, base64.RawURLEncoding.EncodeToString(nonce), base64.RawURLEncoding.EncodeToString(ciphertext), base64.RawURLEncoding.EncodeToString(tag), base64.RawURLEncoding.EncodeToString(wrapped)}, "."), nil
}

func agentOAuthCallbackEnvelopeAAD(input AgentOAuthCallbackEnvelopeInput) string {
	return strings.Join([]string{"dipole.agent.oauth-callback-handoff.v1", input.HandoffUUID, input.TransactionUUID, input.OwnerUserUUID, input.Issuer, input.RedirectURI, input.AuthorizationCodeSHA256, input.RuntimeKeyID, input.ExpiresAtRFC3339Millis}, "\n")
}

func validAgentOAuthCallbackEnvelopeInput(input AgentOAuthCallbackEnvelopeInput) bool {
	return validAgentOAuthCallbackEnvelopeIdentifier(input.HandoffUUID, 64) && validAgentOAuthCallbackEnvelopeIdentifier(input.TransactionUUID, 64) &&
		validAgentOAuthCallbackEnvelopeIdentifier(input.OwnerUserUUID, 64) && validAgentOAuthCallbackEnvelopeURL(input.Issuer) && validAgentOAuthCallbackEnvelopeURL(input.RedirectURI) &&
		len(input.AuthorizationCode) > 0 && len(input.AuthorizationCode) <= 4096 && utf8.ValidString(input.AuthorizationCode) && strings.IndexByte(input.AuthorizationCode, 0) < 0 &&
		len(input.AuthorizationCodeSHA256) == 64 && strings.Trim(input.AuthorizationCodeSHA256, "0123456789abcdef") == "" &&
		sha256Hex(input.AuthorizationCode) == input.AuthorizationCodeSHA256 && validAgentOAuthCallbackEnvelopeIdentifier(input.RuntimeKeyID, 128) && validAgentOAuthCallbackEnvelopeExpiry(input.ExpiresAtRFC3339Millis)
}

func validAgentOAuthCallbackEnvelopeIdentifier(value string, limit int) bool {
	return value != "" && len(value) <= limit && value == strings.TrimSpace(value) && !strings.ContainsAny(value, "\n\r")
}
func validAgentOAuthCallbackEnvelopeURL(value string) bool {
	return strings.HasPrefix(value, "https://") && len(value) <= 2048 && value == strings.TrimSpace(value) && !strings.ContainsAny(value, "?#@\n\r")
}
func sha256Hex(value string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(value))) }
func validAgentOAuthCallbackEnvelopeExpiry(value string) bool {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	return err == nil && parsed.UTC().Format("2006-01-02T15:04:05.000Z") == value
}

func parseAgentOAuthRuntimePublicKey(raw []byte) (*rsa.PublicKey, error) {
	block, rest := pem.Decode(raw)
	if block == nil || len(strings.TrimSpace(string(rest))) != 0 || block.Type != "PUBLIC KEY" {
		return nil, errors.New("invalid public key")
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	key, ok := parsed.(*rsa.PublicKey)
	if !ok || key.Size() < 256 {
		return nil, errors.New("invalid RSA public key")
	}
	return key, nil
}

func clearAgentOAuthCallbackBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}

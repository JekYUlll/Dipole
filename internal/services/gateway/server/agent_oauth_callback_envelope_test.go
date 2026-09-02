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
	"strings"
	"testing"
)

func TestSealAgentOAuthCallbackCodeBindsRuntimeOnlyEnvelope(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	publicDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}
	publicPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER})
	code := "oauth-code-0123456789"
	input := AgentOAuthCallbackEnvelopeInput{HandoffUUID: strings.Repeat("a", 22), TransactionUUID: strings.Repeat("b", 22), OwnerUserUUID: "U100", Issuer: "https://auth.example.com", RedirectURI: "https://dipole.example.com/oauth/callback", AuthorizationCode: code, AuthorizationCodeSHA256: sha256Hex(code), RuntimeKeyID: "oauth-runtime-2026-08", ExpiresAtRFC3339Millis: "2026-08-31T00:10:00.000Z"}
	envelope, err := SealAgentOAuthCallbackCode(input, publicPEM)
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	if strings.Contains(envelope, code) {
		t.Fatal("envelope contains plaintext authorization code")
	}
	parts := strings.Split(envelope, ".")
	if len(parts) != 5 || parts[0] != "v1" {
		t.Fatalf("unexpected envelope: %q", envelope)
	}
	nonce, ciphertext, tag, wrapped := decodeEnvelopeParts(t, parts)
	dataKey, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, privateKey, wrapped, nil)
	if err != nil {
		t.Fatalf("unwrap key: %v", err)
	}
	defer clearAgentOAuthCallbackBytes(dataKey)
	block, err := aes.NewCipher(dataKey)
	if err != nil {
		t.Fatalf("cipher: %v", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatalf("GCM: %v", err)
	}
	sealed := append(append([]byte(nil), ciphertext...), tag...)
	plaintext, err := gcm.Open(nil, nonce, sealed, []byte(agentOAuthCallbackEnvelopeAAD(input)))
	if err != nil || string(plaintext) != code {
		t.Fatalf("open=%q err=%v", plaintext, err)
	}
	input.OwnerUserUUID = "U200"
	if _, err := gcm.Open(nil, nonce, sealed, []byte(agentOAuthCallbackEnvelopeAAD(input))); err == nil {
		t.Fatal("expected changed owner AAD to fail")
	}
}

func TestSealAgentOAuthCallbackCodeRejectsInvalidBinding(t *testing.T) {
	_, err := SealAgentOAuthCallbackCode(AgentOAuthCallbackEnvelopeInput{AuthorizationCode: "code"}, []byte("not a PEM"))
	if !errors.Is(err, ErrAgentOAuthCallbackEnvelopeInvalid) {
		t.Fatalf("error=%v", err)
	}
}

func decodeEnvelopeParts(t *testing.T, parts []string) ([]byte, []byte, []byte, []byte) {
	t.Helper()
	decoded := make([][]byte, 4)
	for index, value := range parts[1:] {
		var err error
		decoded[index], err = base64.RawURLEncoding.DecodeString(value)
		if err != nil {
			t.Fatalf("decode %d: %v", index, err)
		}
	}
	return decoded[0], decoded[1], decoded[2], decoded[3]
}

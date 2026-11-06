// 敏感配置加密：AES-256-GCM。
// 供应商 API Key / 工具认证配置落库前加密，主密钥来自 AGENT_SECRET_KEY。
package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
)

// Encryptor AES-GCM 加解密器。
type Encryptor struct {
	gcm cipher.AEAD
}

// NewEncryptor 用主密钥派生 AES 密钥（sha256）。
func NewEncryptor(masterKey string) (*Encryptor, error) {
	if masterKey == "" {
		return nil, fmt.Errorf("secret_key is empty")
	}
	key := sha256.Sum256([]byte(masterKey))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("new gcm: %w", err)
	}
	return &Encryptor{gcm: gcm}, nil
}

// Encrypt 加密 → base64(nonce+ciphertext)。
func (e *Encryptor) Encrypt(plain string) (string, error) {
	if plain == "" {
		return "", nil
	}
	nonce := make([]byte, e.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("read nonce: %w", err)
	}
	ct := e.gcm.Seal(nonce, nonce, []byte(plain), nil)
	return base64.StdEncoding.EncodeToString(ct), nil
}

// Decrypt 解密 base64(nonce+ciphertext)。
func (e *Encryptor) Decrypt(encoded string) (string, error) {
	if encoded == "" {
		return "", nil
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("base64 decode: %w", err)
	}
	if len(raw) < e.gcm.NonceSize() {
		return "", fmt.Errorf("ciphertext too short")
	}
	plain, err := e.gcm.Open(nil, raw[:e.gcm.NonceSize()], raw[e.gcm.NonceSize():], nil)
	if err != nil {
		return "", fmt.Errorf("gcm open: %w", err)
	}
	return string(plain), nil
}

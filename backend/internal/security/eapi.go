package security

import (
	"crypto/aes"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"strings"
)

const eapiKey = "e82ckenh8dichen8"

func BuildEAPIParams(path string, payload string) (string, error) {
	normalized := strings.Replace(path, "/eapi/", "/api/", 1)
	digest := md5.Sum([]byte("nobody" + normalized + "use" + payload + "md5forencrypt"))
	digestHex := hex.EncodeToString(digest[:])
	plain := fmt.Sprintf("%s-36cd479b6b5-%s-36cd479b6b5-%s", normalized, payload, digestHex)
	encrypted, err := aesECBEncryptPKCS7([]byte(plain), []byte(eapiKey))
	if err != nil {
		return "", err
	}
	return strings.ToLower(hex.EncodeToString(encrypted)), nil
}

func aesECBEncryptPKCS7(src, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	blockSize := block.BlockSize()
	padded := pkcs7Pad(src, blockSize)
	dst := make([]byte, len(padded))
	for i := 0; i < len(padded); i += blockSize {
		block.Encrypt(dst[i:i+blockSize], padded[i:i+blockSize])
	}
	return dst, nil
}

func pkcs7Pad(src []byte, blockSize int) []byte {
	padLen := blockSize - len(src)%blockSize
	out := make([]byte, len(src)+padLen)
	copy(out, src)
	for i := len(src); i < len(out); i++ {
		out[i] = byte(padLen)
	}
	return out
}

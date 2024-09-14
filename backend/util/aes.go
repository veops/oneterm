package util

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"

	"github.com/veops/oneterm/conf"
)

var (
	key, iv []byte
)

func init() {
	key = []byte(conf.Cfg.Auth.Aes.Key)
	iv = []byte(conf.Cfg.Auth.Aes.Iv)
}

func EncryptAES(plainText string) string {
	block, _ := aes.NewCipher(key)
	bs := []byte(plainText)
	bs = paddingPKCS7(bs, aes.BlockSize)

	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(bs, bs)

	return base64.StdEncoding.EncodeToString(bs)
}

func DecryptAES(cipherText string) string {
	bs, _ := base64.StdEncoding.DecodeString(cipherText)
	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err)
	}

	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(bs, bs)

	return string(unPaddingPKCS7(bs))
}

func paddingPKCS7(plaintext []byte, blockSize int) []byte {
	paddingSize := blockSize - len(plaintext)%blockSize
	paddingText := bytes.Repeat([]byte{byte(paddingSize)}, paddingSize)
	return append(plaintext, paddingText...)
}

func unPaddingPKCS7(s []byte) []byte {
	length := len(s)
	if length == 0 {
		return s
	}
	unPadding := int(s[length-1])
	return s[:(length - unPadding)]
}

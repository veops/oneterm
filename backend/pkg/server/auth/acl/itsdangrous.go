package acl

import (
	"bytes"
	"compress/zlib"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"hash"
	"io"
	"strings"
)

// SigningAlgorithm provides interfaces to generate and verify signature
type SigningAlgorithm interface {
	GetSignature(key, value string) []byte
	VerifySignature(key, value string, sig []byte) bool
}

// HMACAlgorithm provides signature generation using HMACs.
type HMACAlgorithm struct {
	DigestMethod func() hash.Hash
}

// GetSignature returns the signature for the given key and value.
func (a *HMACAlgorithm) GetSignature(key, value string) []byte {
	//a.DigestMethod().Reset()
	h := hmac.New(a.DigestMethod, []byte(key))
	h.Write([]byte(value))
	return h.Sum(nil)
}

// VerifySignature verifies the given signature matches the expected signature.
func (a *HMACAlgorithm) VerifySignature(key, value string, sig []byte) bool {
	eq := subtle.ConstantTimeCompare(sig, []byte(a.GetSignature(key, value)))
	return eq == 1
}

type Signature struct {
	SecretKey     string
	Sep           string
	Salt          string
	KeyDerivation string
	DigestMethod  func() hash.Hash
	Algorithm     SigningAlgorithm
}

// Unsign the given string.
func (s *Signature) Unsign(signed string) (content []byte, err error) {
	if !strings.Contains(signed, s.Sep) {
		err = fmt.Errorf("no %s found in value", s.Sep)
		return
	}

	li := strings.LastIndex(signed, s.Sep)
	value, sig := signed[:li], signed[li+len(s.Sep):]

	if ok, _ := s.Verify(value, sig); ok {
		//c, err := base64Decode(strings.Split(strings.Trim(value, "."), ".")[0])
		var c []byte
		c, err = base64.RawURLEncoding.DecodeString(strings.Split(strings.Trim(value, "."), ".")[0])
		if err != nil {
			return
		}

		var r io.ReadCloser
		r, err = zlib.NewReader(bytes.NewReader(c))
		if err != nil {
			return
		}
		return io.ReadAll(r)
	}
	err = fmt.Errorf("signature %s does not match", sig)
	return
}

func (s *Signature) Verify(value, sig string) (bool, error) {
	key, err := s.DeriveKey()
	if err != nil {
		return false, err
	}

	signed, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil {
		return false, err
	}

	return s.Algorithm.VerifySignature(key, value, signed), nil
}

func (s *Signature) DeriveKey() (string, error) {
	var key string
	var err error

	switch s.KeyDerivation {
	case "hmac":
		h := hmac.New(sha1.New, []byte(s.SecretKey))
		h.Write([]byte(s.Salt))

		key = string(h.Sum(nil))
	case "none":
		key = s.SecretKey
	default:
		key, err = "", errors.New("unknown key derivation method")
	}

	return key, err
}

func NewSignature(secret, salt, sep, derivation string, digest func() hash.Hash, algo SigningAlgorithm) *Signature {
	if salt == "" {
		salt = "itsdangerous.Signer"
	}
	if sep == "" {
		sep = "."
	}
	if derivation == "" {
		derivation = "hmac"
	}
	if digest == nil {
		digest = sha1.New
	}
	if algo == nil {
		algo = &HMACAlgorithm{DigestMethod: digest}
	}

	return &Signature{
		SecretKey:     secret,
		Salt:          salt,
		Sep:           sep,
		KeyDerivation: derivation,
		DigestMethod:  digest,
		Algorithm:     algo,
	}
}

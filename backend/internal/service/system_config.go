package service

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"strings"

	"golang.org/x/crypto/ssh"

	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/repository"
	"github.com/veops/oneterm/pkg/utils"
)

type SystemConfigService struct {
	repo repository.SystemConfigRepository
}

func NewSystemConfigService() *SystemConfigService {
	return &SystemConfigService{
		repo: repository.NewSystemConfigRepository(),
	}
}

// GetSSHPrivateKey gets SSH private key from database and decrypts it
func (s *SystemConfigService) GetSSHPrivateKey() (string, error) {
	ctx := context.Background()
	config, err := s.repo.GetByKey(ctx, model.SysConfigSSHPrivateKey)
	if err != nil {
		return "", err
	}
	// Decrypt the private key
	return utils.DecryptAES(config.Value), nil
}

// SetSSHPrivateKey encrypts and sets SSH private key to database
func (s *SystemConfigService) SetSSHPrivateKey(privateKey string) error {
	// Encrypt the private key before storing
	encryptedPrivateKey := utils.EncryptAES(privateKey)
	ctx := context.Background()
	return s.repo.SetByKey(ctx, model.SysConfigSSHPrivateKey, encryptedPrivateKey)
}

// GenerateSSHKeyPair generates a new ED25519 SSH key pair
func (s *SystemConfigService) GenerateSSHKeyPair() (privateKey string, publicKey string, err error) {
	// Generate ED25519 key pair
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate ED25519 key: %w", err)
	}

	// Convert to SSH private key format
	sshPrivKey, err := ssh.MarshalPrivateKey(privKey, "")
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal private key: %w", err)
	}

	// Encode private key to PEM format
	privateKeyPEM := pem.EncodeToMemory(sshPrivKey)

	// Convert to SSH public key format
	sshPubKey, err := ssh.NewPublicKey(pubKey)
	if err != nil {
		return "", "", fmt.Errorf("failed to create SSH public key: %w", err)
	}

	publicKeyStr := string(ssh.MarshalAuthorizedKey(sshPubKey))

	return string(privateKeyPEM), strings.TrimSpace(publicKeyStr), nil
}

// EnsureSSHPrivateKey ensures SSH private key exists, generates one if not
func (s *SystemConfigService) EnsureSSHPrivateKey() (string, error) {
	// Try to get existing key from database
	privateKey, err := s.GetSSHPrivateKey()
	if err == nil && privateKey != "" {
		return privateKey, nil
	}

	// If not found or error, check if it's a "not found" error
	if err != nil && !strings.Contains(err.Error(), "record not found") {
		return "", fmt.Errorf("failed to query SSH private key: %w", err)
	}

	// Generate new key pair
	privateKey, _, err = s.GenerateSSHKeyPair()
	if err != nil {
		return "", fmt.Errorf("failed to generate SSH key pair: %w", err)
	}

	// Save to database
	err = s.SetSSHPrivateKey(privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to save SSH private key: %w", err)
	}

	return privateKey, nil
}

// MigrateFromConfig migrates SSH private key from config file to database
func (s *SystemConfigService) MigrateFromConfig(configPrivateKey string) error {
	if configPrivateKey == "" {
		return nil
	}

	// Check if database already has a key
	_, err := s.GetSSHPrivateKey()
	if err == nil {
		// Key already exists in database, skip migration
		return nil
	}

	if !strings.Contains(err.Error(), "record not found") {
		return fmt.Errorf("failed to check existing SSH private key: %w", err)
	}

	// Validate the private key from config
	_, err = ssh.ParsePrivateKey([]byte(configPrivateKey))
	if err != nil {
		return fmt.Errorf("invalid SSH private key in config: %w", err)
	}

	// Save to database
	return s.SetSSHPrivateKey(configPrivateKey)
}
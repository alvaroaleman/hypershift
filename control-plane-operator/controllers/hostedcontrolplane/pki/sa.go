package pki

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"

	"github.com/openshift/hypershift/control-plane-operator/controllers/hostedcontrolplane/config"
	"github.com/openshift/hypershift/support/certs"
)

func ReconcileServiceAccountSigningKeySecret(secret *corev1.Secret, ownerRef config.OwnerRef) error {
	expectedKeys := []string{ServiceSignerPrivateKey, ServiceSignerPublicKey}
	if secret != nil && SecretUpToDate(secret, expectedKeys) {
		return nil
	}
	ownerRef.ApplyTo(secret)
	secret.Type = corev1.SecretTypeOpaque
	key, err := certs.PrivateKey()
	if err != nil {
		return fmt.Errorf("failed generating a private key: %w", err)
	}
	keyBytes := certs.PrivateKeyToPem(key)
	publicKeyBytes, err := certs.PublicKeyToPem(&key.PublicKey)
	if err != nil {
		return fmt.Errorf("failed to generate public key from private key: %w", err)
	}
	if secret.Data == nil {
		secret.Data = map[string][]byte{}
	}
	secret.Data[ServiceSignerPrivateKey] = keyBytes
	secret.Data[ServiceSignerPublicKey] = publicKeyBytes
	return nil
}

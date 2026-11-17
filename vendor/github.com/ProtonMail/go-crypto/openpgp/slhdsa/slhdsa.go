// Package slhdsa implements SLH-DSA-SHAKE for OpenPGP,
// according to https://www.rfc-editor.org/rfc/rfc9980.html#name-slh-dsa-2.
package slhdsa

import (
	goerrors "errors"
	"fmt"
	"io"

	"github.com/ProtonMail/go-crypto/openpgp/errors"
	"github.com/cloudflare/circl/sign"
)

type PublicKey struct {
	AlgId        uint8
	Slhdsa       sign.Scheme
	PublicSlhdsa sign.PublicKey
}

type PrivateKey struct {
	PublicKey
	SecretSlhdsa sign.PrivateKey
}

// GenerateKey generates a SLH-DSA key.
func GenerateKey(rand io.Reader, algId uint8, scheme sign.Scheme) (priv *PrivateKey, err error) {
	priv = new(PrivateKey)

	priv.PublicKey.AlgId = algId
	priv.PublicKey.Slhdsa = scheme

	keySeed := make([]byte, scheme.SeedSize())
	if _, err = rand.Read(keySeed); err != nil {
		return nil, err
	}
	priv.PublicKey.PublicSlhdsa, priv.SecretSlhdsa = priv.PublicKey.Slhdsa.DeriveKey(keySeed)

	return priv, nil
}

// Sign generates a SLH-DSA signature.
func Sign(priv *PrivateKey, message []byte) (signature []byte, err error) {
	signature, err = priv.SecretSlhdsa.Sign(nil, message, nil)
	if err != nil {
		return nil, fmt.Errorf("slhdsa: unable to sign with SLH-DSA: %s", err)
	}
	if signature == nil {
		return nil, goerrors.New("slhdsa: unable to sign with SLH-DSA")
	}

	return signature, nil
}

// Verify verifies the SLH-DSA signature.
func Verify(pub *PublicKey, message, dSig []byte) bool {
	return pub.Slhdsa.Verify(pub.PublicSlhdsa, message, dSig, nil)
}

// Validate checks that the public key corresponds to the private key
func Validate(priv *PrivateKey) (err error) {
	if !priv.PublicSlhdsa.Equal(priv.SecretSlhdsa.Public()) {
		return errors.KeyInvalidError("slhdsa: invalid public key")
	}

	return nil
}

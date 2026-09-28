// Package mlkem_ecdh implements hybrid ML-KEM + ECDH encryption, suitable for OpenPGP, experimental.
// It follows the spec https://www.rfc-editor.org/rfc/rfc9980.html#name-composite-kem-schemes
package mlkem_ecdh

import (
	goerrors "errors"
	"fmt"
	"io"

	"golang.org/x/crypto/sha3"

	"github.com/ProtonMail/go-crypto/openpgp/aes/keywrap"
	"github.com/ProtonMail/go-crypto/openpgp/errors"
	"github.com/ProtonMail/go-crypto/openpgp/internal/ecc"
	"github.com/cloudflare/circl/kem"
)

const (
	maxSessionKeyLength = 64
	MlKemSeedLen        = 64
	domSep              = "OpenPGPCompositeKDFv1"
)

type PublicKey struct {
	AlgId       uint8
	Curve       ecc.ECDHCurve
	Mlkem       kem.Scheme
	PublicMlkem kem.PublicKey
	PublicPoint []byte
}

type PrivateKey struct {
	PublicKey
	SecretEc        []byte
	SecretMlkem     kem.PrivateKey
	SecretMlkemSeed []byte
}

// GenerateKey implements ML-KEM + ECC key generation as specified in
// https://www.rfc-editor.org/rfc/rfc9980.html#name-key-generation-procedure
func GenerateKey(rand io.Reader, algId uint8, c ecc.ECDHCurve, k kem.Scheme) (priv *PrivateKey, err error) {
	priv = new(PrivateKey)

	priv.PublicKey.AlgId = algId
	priv.PublicKey.Curve = c
	priv.PublicKey.Mlkem = k

	priv.PublicKey.PublicPoint, priv.SecretEc, err = c.GenerateECDH(rand)
	if err != nil {
		return nil, err
	}

	seed, err := generateRandomSeed(rand, MlKemSeedLen)
	if err != nil {
		return nil, err
	}

	if err := priv.DeriveMlKemKeys(seed, true); err != nil {
		return nil, err
	}
	return priv, nil
}

// DeriveMlKemKeys derives the ML-KEM keys from the provided seed and stores them inside priv.
func (priv *PrivateKey) DeriveMlKemKeys(seed []byte, overridePublicKey bool) (err error) {
	if len(seed) != MlKemSeedLen {
		return goerrors.New("mlkem_ecdh: ml-kem secret seed has the wrong length")
	}
	priv.SecretMlkemSeed = seed
	publicKey, privateKey := priv.PublicKey.Mlkem.DeriveKeyPair(priv.SecretMlkemSeed)
	if overridePublicKey {
		priv.PublicKey.PublicMlkem = publicKey
	}
	priv.SecretMlkem = privateKey
	return nil
}

// Encrypt implements ML-KEM + ECC encryption as specified in
// https://www.rfc-editor.org/rfc/rfc9980.html#name-encryption-procedure
func Encrypt(rand io.Reader, pub *PublicKey, msg []byte) (kEphemeral, ecEphemeral, ciphertext []byte, err error) {
	if len(msg) > maxSessionKeyLength {
		return nil, nil, nil, goerrors.New("mlkem_ecdh: session key too long")
	}

	if len(msg)%8 != 0 {
		return nil, nil, nil, goerrors.New("mlkem_ecdh: session key not a multiple of 8")
	}

	// EC shared secret derivation
	ecEphemeral, ecSS, err := pub.Curve.Encaps(rand, pub.PublicPoint)
	if err != nil {
		return nil, nil, nil, err
	}

	// ML-KEM shared secret derivation
	kyberSeed, err := generateRandomSeed(rand, pub.Mlkem.EncapsulationSeedSize())
	if err != nil {
		return nil, nil, nil, err
	}

	kEphemeral, kSS, err := pub.Mlkem.EncapsulateDeterministically(pub.PublicMlkem, kyberSeed)
	if err != nil {
		return nil, nil, nil, err
	}

	keyEncryptionKey, err := buildKey(pub, ecSS, ecEphemeral, pub.PublicPoint, kSS)
	if err != nil {
		return nil, nil, nil, err
	}

	if ciphertext, err = keywrap.Wrap(keyEncryptionKey, msg); err != nil {
		return nil, nil, nil, err
	}

	return kEphemeral, ecEphemeral, ciphertext, nil
}

// Decrypt implements ML-KEM + ECC decryption as specified in
// https://www.rfc-editor.org/rfc/rfc9980.html#name-decryption-procedure
func Decrypt(priv *PrivateKey, kEphemeral, ecEphemeral, ciphertext []byte) (msg []byte, err error) {
	// EC shared secret derivation
	ecSS, err := priv.PublicKey.Curve.Decaps(ecEphemeral, priv.SecretEc)
	if err != nil {
		return nil, err
	}

	// ML-KEM shared secret derivation
	kSS, err := priv.PublicKey.Mlkem.Decapsulate(priv.SecretMlkem, kEphemeral)
	if err != nil {
		return nil, err
	}

	kek, err := buildKey(&priv.PublicKey, ecSS, ecEphemeral, priv.PublicPoint, kSS)
	if err != nil {
		return nil, err
	}

	return keywrap.Unwrap(kek, ciphertext)
}

// buildKey implements the composite KDF from
// https://www.rfc-editor.org/rfc/rfc9980.html#name-key-combiner
func buildKey(pub *PublicKey, eccSecretPoint, eccCipherText, eccPublicKey, mlkemKeyShare []byte) ([]byte, error) {
	/// Set the output `ecdhKeyShare` to `eccSecretPoint`
	eccKeyShare := eccSecretPoint

	//   mlkemKeyShare   - the ML-KEM key share encoded as an octet string
	//   algId           - the OpenPGP algorithm ID of the public-key encryption algorithm
	//   eccKeyShare     - the ECDH key share encoded as an octet string
	//   eccCipherText  - the ECDH ciphertext encoded as an octet string
	//   eccPublicKey    - The ECDH public key of the recipient as an octet string

	// SHA3-256(mlkemKeyShare || eccKeyShare || eccCipherText || eccPublicKey ||
	//          algId || "OpenPGPCompositeKDFv1" || len("OpenPGPCompositeKDFv1"))
	h := sha3.New256()
	_, _ = h.Write(mlkemKeyShare)
	_, _ = h.Write(eccKeyShare)
	_, _ = h.Write(eccCipherText)
	_, _ = h.Write(eccPublicKey)
	_, _ = h.Write([]byte{pub.AlgId})
	_, _ = h.Write([]byte(domSep))
	_, _ = h.Write([]byte{byte(len(domSep))})
	return h.Sum(nil), nil
}

// Validate checks that the public key corresponds to the private key
func Validate(priv *PrivateKey) (err error) {
	if err = priv.PublicKey.Curve.ValidateECDH(priv.PublicKey.PublicPoint, priv.SecretEc); err != nil {
		return err
	}

	if !priv.PublicKey.PublicMlkem.Equal(priv.SecretMlkem.Public()) {
		return errors.KeyInvalidError("mlkem_ecdh: invalid public key")
	}

	return
}

// EncodeFields encodes an ML-KEM + ECDH session key encryption fields as
// ephemeral ECDH public key | ML-KEM ciphertext | follow byte length | cipherFunction (v3 only) | encryptedSessionKey
// and writes it to writer.
func EncodeFields(w io.Writer, ec, ml, encryptedSessionKey []byte, cipherFunction byte, v6 bool) (err error) {
	if _, err = w.Write(ec); err != nil {
		return err
	}

	if _, err = w.Write(ml); err != nil {
		return err
	}

	lenAlgorithm := 0
	if !v6 {
		lenAlgorithm = 1
	}

	if _, err = w.Write([]byte{byte(len(encryptedSessionKey) + lenAlgorithm)}); err != nil {
		return err
	}

	if !v6 {
		if _, err = w.Write([]byte{cipherFunction}); err != nil {
			return err
		}
	}

	if _, err = w.Write(encryptedSessionKey); err != nil {
		return err
	}

	return nil
}

// DecodeFields decodes an ML-KEM + ECDH session key encryption fields as
// ephemeral ECDH public key | ML-KEM ciphertext | follow byte length | cipherFunction (v3 only) | encryptedSessionKey.
func DecodeFields(r io.Reader, lenEcc, lenMlkem int, v6 bool) (ephemeralPublicEcc, ephemeralPublicMlKem, encryptedSessionKey []byte, cipherFunction byte, err error) {
	var buf [1]byte

	ephemeralPublicEcc = make([]byte, lenEcc)
	if _, err = io.ReadFull(r, ephemeralPublicEcc); err != nil {
		return
	}

	ephemeralPublicMlKem = make([]byte, lenMlkem)
	if _, err = io.ReadFull(r, ephemeralPublicMlKem); err != nil {
		return
	}

	// A one-octet size of the following fields.
	if _, err = io.ReadFull(r, buf[:]); err != nil {
		return
	}

	followingLen := buf[0]
	// The one-octet algorithm identifier, if it was passed (in the case of a v3 PKESK packet).
	if !v6 {
		if _, err = io.ReadFull(r, buf[:]); err != nil {
			return
		}
		cipherFunction = buf[0]
		followingLen -= 1
	}

	// The encrypted session key.
	encryptedSessionKey = make([]byte, followingLen)
	if _, err = io.ReadFull(r, encryptedSessionKey); err != nil {
		return
	}

	return
}

func generateRandomSeed(rand io.Reader, size int) ([]byte, error) {
	randomBytes := make([]byte, size)
	if _, err := rand.Read(randomBytes); err != nil {
		return nil, fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return randomBytes, nil
}

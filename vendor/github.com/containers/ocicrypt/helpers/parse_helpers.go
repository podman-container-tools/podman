package helpers

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/containers/ocicrypt"
	encconfig "github.com/containers/ocicrypt/config"
	"github.com/containers/ocicrypt/config/pkcs11config"
	"github.com/containers/ocicrypt/crypto/pkcs11"
	encutils "github.com/containers/ocicrypt/utils"
)

// processRecipientKeys sorts the array of recipients by type. Recipients may be either
// x509 certificates, public keys, or PGP public keys identified by email address or name
func processRecipientKeys(recipients []string) ([][]byte, [][]byte, [][]byte, [][]byte, [][]byte, [][]byte, error) {
	var (
		gpgRecipients [][]byte
		pubkeys       [][]byte
		x509s         [][]byte
		pkcs11Pubkeys [][]byte
		pkcs11Yamls   [][]byte
		keyProviders  [][]byte
	)

	for _, recipient := range recipients {

		idx := strings.Index(recipient, ":")
		if idx < 0 {
			return nil, nil, nil, nil, nil, nil, errors.New("Invalid recipient format")
		}

		protocol := recipient[:idx]
		value := recipient[idx+1:]

		switch protocol {
		case "pgp":
			gpgRecipients = append(gpgRecipients, []byte(value))

		case "jwe":
			tmp, err := os.ReadFile(value)
			if err != nil {
				return nil, nil, nil, nil, nil, nil, fmt.Errorf("Unable to read file: %w", err)
			}
			if !encutils.IsPublicKey(tmp) {
				return nil, nil, nil, nil, nil, nil, errors.New("File provided is not a public key")
			}
			pubkeys = append(pubkeys, tmp)

		case "pkcs7":
			tmp, err := os.ReadFile(value)
			if err != nil {
				return nil, nil, nil, nil, nil, nil, fmt.Errorf("Unable to read file: %w", err)
			}
			if !encutils.IsCertificate(tmp) {
				return nil, nil, nil, nil, nil, nil, errors.New("File provided is not an x509 cert")
			}
			x509s = append(x509s, tmp)

		case "pkcs11":
			tmp, err := os.ReadFile(value)
			if err != nil {
				return nil, nil, nil, nil, nil, nil, fmt.Errorf("Unable to read file: %w", err)
			}
			if encutils.IsPkcs11PublicKey(tmp) {
				pkcs11Yamls = append(pkcs11Yamls, tmp)
			} else if encutils.IsPublicKey(tmp) {
				pkcs11Pubkeys = append(pkcs11Pubkeys, tmp)
			} else {
				return nil, nil, nil, nil, nil, nil, errors.New("Provided file is not a public key")
			}

		case "provider":
			keyProviders = append(keyProviders, []byte(value))

		default:
			return nil, nil, nil, nil, nil, nil, errors.New("Provided protocol not recognized")
		}
	}
	return gpgRecipients, pubkeys, x509s, pkcs11Pubkeys, pkcs11Yamls, keyProviders, nil
}

// processx509Certs processes x509 certificate files
func processx509Certs(keys []string) ([][]byte, error) {
	var x509s [][]byte
	for _, key := range keys {
		fileName := strings.Split(key, ":")[0]
		if _, err := os.Stat(fileName); os.IsNotExist(err) {
			continue
		}
		tmp, err := os.ReadFile(fileName)
		if err != nil {
			return nil, fmt.Errorf("Unable to read file: %w", err)
		}
		if !encutils.IsCertificate(tmp) {
			continue
		}
		x509s = append(x509s, tmp)

	}
	return x509s, nil
}

// processPwdString process a password that may be in any of the following formats:
// - file=<passwordfile>
// - pass=<password>
// - fd=<filedescriptor>
// - <password>
func processPwdString(pwdString string) ([]byte, error) {
	if strings.HasPrefix(pwdString, "file=") {
		return os.ReadFile(pwdString[5:])
	} else if strings.HasPrefix(pwdString, "pass=") {
		return []byte(pwdString[5:]), nil
	} else if strings.HasPrefix(pwdString, "fd=") {
		fdStr := pwdString[3:]
		fd, err := strconv.Atoi(fdStr)
		if err != nil {
			return nil, fmt.Errorf("could not parse file descriptor %s: %w", fdStr, err)
		}
		f := os.NewFile(uintptr(fd), "pwdfile")
		if f == nil {
			return nil, fmt.Errorf("%s is not a valid file descriptor", fdStr)
		}
		defer f.Close()
		pwd := make([]byte, 64)
		n, err := f.Read(pwd)
		if err != nil {
			return nil, fmt.Errorf("could not read from file descriptor: %w", err)
		}
		return pwd[:n], nil
	}
	return []byte(pwdString), nil
}

// processPrivateKeyFiles sorts the different types of private key files; private key files may either be
// private keys or GPG private key ring files. The private key files may include the password for the
// private key and take any of the following forms:
// - <filename>
// - <filename>:file=<passwordfile>
// - <filename>:pass=<password>
// - <filename>:fd=<filedescriptor>
// - <filename>:<password>
// - keyprovider:<...>
func processPrivateKeyFiles(keyFilesAndPwds []string) ([][]byte, [][]byte, [][]byte, [][]byte, [][]byte, [][]byte, error) {
	var (
		gpgSecretKeyRingFiles [][]byte
		gpgSecretKeyPasswords [][]byte
		privkeys              [][]byte
		privkeysPasswords     [][]byte
		pkcs11Yamls           [][]byte
		keyProviders          [][]byte
		err                   error
	)
	// keys needed for decryption in case of adding a recipient
	for _, keyfileAndPwd := range keyFilesAndPwds {
		var password []byte

		// treat "provider" protocol separately
		if strings.HasPrefix(keyfileAndPwd, "provider:") {
			keyProviders = append(keyProviders, []byte(keyfileAndPwd[len("provider:"):]))
			continue
		}
		parts := strings.Split(keyfileAndPwd, ":")
		if len(parts) == 2 {
			password, err = processPwdString(parts[1])
			if err != nil {
				return nil, nil, nil, nil, nil, nil, err
			}
		}

		keyfile := parts[0]
		tmp, err := os.ReadFile(keyfile)
		if err != nil {
			return nil, nil, nil, nil, nil, nil, err
		}
		isPrivKey, err := encutils.IsPrivateKey(tmp, password)
		if encutils.IsPasswordError(err) {
			return nil, nil, nil, nil, nil, nil, err
		}

		if encutils.IsPkcs11PrivateKey(tmp) {
			pkcs11Yamls = append(pkcs11Yamls, tmp)
		} else if isPrivKey {
			privkeys = append(privkeys, tmp)
			privkeysPasswords = append(privkeysPasswords, password)
		} else if encutils.IsGPGPrivateKeyRing(tmp) {
			gpgSecretKeyRingFiles = append(gpgSecretKeyRingFiles, tmp)
			gpgSecretKeyPasswords = append(gpgSecretKeyPasswords, password)
		} else {
			// ignore if file is not recognized, so as not to error if additional
			// metadata/cert files exists
			continue
		}
	}
	return gpgSecretKeyRingFiles, gpgSecretKeyPasswords, privkeys, privkeysPasswords, pkcs11Yamls, keyProviders, nil
}

// CreateDecryptCryptoConfig creates the CryptoConfig object that contains the necessary
// information to perform decryption from command line options.
func CreateDecryptCryptoConfig(keys []string, decRecipients []string) (encconfig.CryptoConfig, error) {
	ccs := []encconfig.CryptoConfig{}

	// x509 cert is needed for PKCS7 decryption
	_, _, x509s, _, _, _, err := processRecipientKeys(decRecipients)
	if err != nil {
		return encconfig.CryptoConfig{}, err
	}

	// x509 certs can also be passed in via keys
	x509FromKeys, err := processx509Certs(keys)
	if err != nil {
		return encconfig.CryptoConfig{}, err
	}
	x509s = append(x509s, x509FromKeys...)

	gpgSecretKeyRingFiles, gpgSecretKeyPasswords, privKeys, privKeysPasswords, pkcs11Yamls, keyProviders, err := processPrivateKeyFiles(keys)
	if err != nil {
		return encconfig.CryptoConfig{}, err
	}

	if len(gpgSecretKeyRingFiles) > 0 {
		gpgCc, err := encconfig.DecryptWithGpgPrivKeys(gpgSecretKeyRingFiles, gpgSecretKeyPasswords)
		if err != nil {
			return encconfig.CryptoConfig{}, err
		}
		ccs = append(ccs, gpgCc)
	}

	/* TODO: Add in GPG client query for secret keys in the future.
	_, err = createGPGClient(context)
	gpgInstalled := err == nil
	if gpgInstalled {
		if len(gpgSecretKeyRingFiles) == 0 && len(privKeys) == 0 && len(pkcs11Yamls) == 0 && len(keyProviders) == 0 && descs != nil {
			// Get pgp private keys from keyring only if no private key was passed
			gpgPrivKeys, gpgPrivKeyPasswords, err := getGPGPrivateKeys(context, gpgSecretKeyRingFiles, descs, true)
			if err != nil {
				return encconfig.CryptoConfig{}, err
			}

			gpgCc, err := encconfig.DecryptWithGpgPrivKeys(gpgPrivKeys, gpgPrivKeyPasswords)
			if err != nil {
				return encconfig.CryptoConfig{}, err
			}
			ccs = append(ccs, gpgCc)

		} else if len(gpgSecretKeyRingFiles) > 0 {
			gpgCc, err := encconfig.DecryptWithGpgPrivKeys(gpgSecretKeyRingFiles, gpgSecretKeyPasswords)
			if err != nil {
				return encconfig.CryptoConfig{}, err
			}
			ccs = append(ccs, gpgCc)

		}
	}
	*/

	if len(x509s) > 0 {
		x509sCc, err := encconfig.DecryptWithX509s(x509s)
		if err != nil {
			return encconfig.CryptoConfig{}, err
		}
		ccs = append(ccs, x509sCc)
	}
	if len(privKeys) > 0 {
		privKeysCc, err := encconfig.DecryptWithPrivKeys(privKeys, privKeysPasswords)
		if err != nil {
			return encconfig.CryptoConfig{}, err
		}
		ccs = append(ccs, privKeysCc)
	}
	if len(pkcs11Yamls) > 0 {
		p11conf, err := pkcs11config.GetUserPkcs11Config()
		if err != nil {
			return encconfig.CryptoConfig{}, err
		}
		pkcs11PrivKeysCc, err := encconfig.DecryptWithPkcs11Yaml(p11conf, pkcs11Yamls)
		if err != nil {
			return encconfig.CryptoConfig{}, err
		}
		ccs = append(ccs, pkcs11PrivKeysCc)
	}
	if len(keyProviders) > 0 {
		keyProviderCc, err := encconfig.DecryptWithKeyProvider(keyProviders)
		if err != nil {
			return encconfig.CryptoConfig{}, err
		}
		ccs = append(ccs, keyProviderCc)
	}
	return encconfig.CombineCryptoConfigs(ccs), nil
}

// CreateCryptoConfig from the list of recipient strings and list of key paths of private keys
func CreateCryptoConfig(recipients []string, keys []string) (encconfig.CryptoConfig, error) {
	var decryptCc *encconfig.CryptoConfig
	ccs := []encconfig.CryptoConfig{}
	if len(keys) > 0 {
		dcc, err := CreateDecryptCryptoConfig(keys, []string{})
		if err != nil {
			return encconfig.CryptoConfig{}, err
		}
		decryptCc = &dcc
		ccs = append(ccs, dcc)
	}

	if len(recipients) > 0 {
		gpgRecipients, pubKeys, x509s, pkcs11Pubkeys, pkcs11Yamls, keyProvider, err := processRecipientKeys(recipients)
		if err != nil {
			return encconfig.CryptoConfig{}, err
		}
		encryptCcs := []encconfig.CryptoConfig{}

		// Create GPG client with guessed GPG version and default homedir
		gpgClient, err := ocicrypt.NewGPGClient("", "")
		gpgInstalled := err == nil
		if len(gpgRecipients) > 0 && gpgInstalled {
			gpgPubRingFile, err := gpgClient.ReadGPGPubRingFile()
			if err != nil {
				return encconfig.CryptoConfig{}, err
			}

			gpgCc, err := encconfig.EncryptWithGpg(gpgRecipients, gpgPubRingFile)
			if err != nil {
				return encconfig.CryptoConfig{}, err
			}
			encryptCcs = append(encryptCcs, gpgCc)
		}

		// Create Encryption Crypto Config
		if len(x509s) > 0 {
			pkcs7Cc, err := encconfig.EncryptWithPkcs7(x509s)
			if err != nil {
				return encconfig.CryptoConfig{}, err
			}
			encryptCcs = append(encryptCcs, pkcs7Cc)
		}
		if len(pubKeys) > 0 {
			jweCc, err := encconfig.EncryptWithJwe(pubKeys)
			if err != nil {
				return encconfig.CryptoConfig{}, err
			}
			encryptCcs = append(encryptCcs, jweCc)
		}
		var p11conf *pkcs11.Pkcs11Config
		if len(pkcs11Yamls) > 0 || len(pkcs11Pubkeys) > 0 {
			p11conf, err = pkcs11config.GetUserPkcs11Config()
			if err != nil {
				return encconfig.CryptoConfig{}, err
			}
			pkcs11Cc, err := encconfig.EncryptWithPkcs11(p11conf, pkcs11Pubkeys, pkcs11Yamls)
			if err != nil {
				return encconfig.CryptoConfig{}, err
			}
			encryptCcs = append(encryptCcs, pkcs11Cc)
		}

		if len(keyProvider) > 0 {
			keyProviderCc, err := encconfig.EncryptWithKeyProvider(keyProvider)
			if err != nil {
				return encconfig.CryptoConfig{}, err
			}
			encryptCcs = append(encryptCcs, keyProviderCc)
		}
		ecc := encconfig.CombineCryptoConfigs(encryptCcs)
		if decryptCc != nil {
			ecc.EncryptConfig.AttachDecryptConfig(decryptCc.DecryptConfig)
		}
		ccs = append(ccs, ecc)
	}

	if len(ccs) > 0 {
		return encconfig.CombineCryptoConfigs(ccs), nil
	}
	return encconfig.CryptoConfig{}, nil
}

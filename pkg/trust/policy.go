package trust

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/sirupsen/logrus"
	"go.podman.io/storage/pkg/fileutils"
)

// policyContent is the overall structure of a policy.json file (= c/image/v5/signature.Policy)
type policyContent struct {
	Default    []repoContent     `json:"default"`
	Transports transportsContent `json:"transports,omitempty"`
}

// transportsContent contains policies for individual transports (= c/image/v5/signature.Policy.Transports)
type transportsContent map[string]repoMap

// repoMap maps a scope name to requirements that apply to that scope (= c/image/v5/signature.PolicyTransportScopes)
type repoMap map[string][]repoContent

// repoContent is a single policy requirement (one of possibly several for a scope), representing all of the individual alternatives in a single merged struct
// (= c/image/v5/signature.{PolicyRequirement,pr*})
type repoContent struct {
	Type           string          `json:"type"`
	KeyType        string          `json:"keyType,omitempty"`
	KeyPath        string          `json:"keyPath,omitempty"`
	KeyPaths       []string        `json:"keyPaths,omitempty"`
	KeyData        string          `json:"keyData,omitempty"`
	SignedIdentity json.RawMessage `json:"signedIdentity,omitempty"`
}

// genericPolicyContent is the overall structure of a policy.json file (= c/image/v5/signature.Policy), using generic data for individual requirements.
type genericPolicyContent struct {
	Default    json.RawMessage          `json:"default"`
	Transports genericTransportsContent `json:"transports,omitempty"`
}

// genericTransportsContent contains policies for individual transports (= c/image/v5/signature.Policy.Transports), using generic data for individual requirements.
type genericTransportsContent map[string]genericRepoMap

// genericRepoMap maps a scope name to requirements that apply to that scope (= c/image/v5/signature.PolicyTransportScopes)
type genericRepoMap map[string]json.RawMessage

// gpgIDReader returns GPG key IDs of keys stored at the provided path.
// It exists only for tests, production code should always use getGPGIdFromKeyPath.
type gpgIDReader func(string) []string

// createTmpFile creates a temp file under dir and writes the content into it
func createTmpFile(dir, pattern string, content []byte) (string, error) {
	tmpfile, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return "", err
	}
	defer tmpfile.Close()

	if _, err := tmpfile.Write(content); err != nil {
		return "", err
	}
	return tmpfile.Name(), nil
}

// getGPGIdFromKeyPath returns GPG key IDs of keys stored at the provided path.
func getGPGIdFromKeyPath(path string) []string {
	cmd := exec.Command("gpg2", "--with-colons", path)
	results, err := cmd.Output()
	if err != nil {
		logrus.Errorf("Getting key identity: %s", err)
		return nil
	}
	return parseUids(results)
}

// getGPGIdFromKeyData returns GPG key IDs of keys in the provided keyring.
func getGPGIdFromKeyData(idReader gpgIDReader, key string) []string {
	decodeKey, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		logrus.Errorf("%s, error decoding key data", err)
		return nil
	}
	tmpfileName, err := createTmpFile("", "", decodeKey)
	if err != nil {
		logrus.Errorf("Creating key date temp file %s", err)
	}
	defer os.Remove(tmpfileName)
	return idReader(tmpfileName)
}

func parseUids(colonDelimitKeys []byte) []string {
	var parseduids []string
	scanner := bufio.NewScanner(bytes.NewReader(colonDelimitKeys))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "uid:") || strings.HasPrefix(line, "pub:") {
			uid := strings.Split(line, ":")[9]
			if uid == "" {
				continue
			}
			parseduid := uid
			if ltidx := strings.Index(uid, "<"); ltidx != -1 {
				subuid := parseduid[ltidx+1:]
				if before, _, ok := strings.Cut(subuid, ">"); ok {
					parseduid = before
				}
			}
			parseduids = append(parseduids, parseduid)
		}
	}
	return parseduids
}

// getPolicy parses policy.json into policyContent.
func getPolicy(policyPath string) (policyContent, error) {
	var policyContentStruct policyContent
	policyContent, err := os.ReadFile(policyPath)
	if err != nil {
		return policyContentStruct, fmt.Errorf("unable to read policy file: %w", err)
	}
	if err := json.Unmarshal(policyContent, &policyContentStruct); err != nil {
		return policyContentStruct, fmt.Errorf("could not parse trust policies from %s: %w", policyPath, err)
	}
	return policyContentStruct, nil
}

var typeDescription = map[string]string{"insecureAcceptAnything": "accept", "signedBy": "signed", "sigstoreSigned": "sigstoreSigned", "reject": "reject"}

func trustTypeDescription(trustType string) string {
	trustDescription, exist := typeDescription[trustType]
	if !exist {
		logrus.Warnf("Invalid trust type %s", trustType)
	}
	return trustDescription
}

// AddPolicyEntriesInput collects some parameters to AddPolicyEntries,
// primarily so that the callers use named values instead of just strings in a sequence.
type AddPolicyEntriesInput struct {
	Scope       string // "default" or a docker/atomic scope name
	Type        string
	PubKeyFiles []string // For signature enforcement types, paths to public keys files (where the image needs to be signed by at least one key from _each_ of the files). File format depends on Type.
}

// AddPolicyEntries adds one or more policy entries necessary to implement AddPolicyEntriesInput.
func AddPolicyEntries(policyPath string, input AddPolicyEntriesInput) error {
	var (
		policyContentStruct genericPolicyContent
		newReposContent     []repoContent
	)
	trustType := input.Type
	if trustType == "accept" {
		trustType = "insecureAcceptAnything"
	}
	pubkeysfile := input.PubKeyFiles

	// The error messages in validation failures use input.Type instead of trustType to match the user’s input.
	switch trustType {
	case "insecureAcceptAnything", "reject":
		if len(pubkeysfile) != 0 {
			return fmt.Errorf("%d public keys unexpectedly provided for trust type %v", len(pubkeysfile), input.Type)
		}
		newReposContent = append(newReposContent, repoContent{Type: trustType})

	case "signedBy":
		if len(pubkeysfile) == 0 {
			return errors.New("at least one public key must be defined for type 'signedBy'")
		}
		for _, filepath := range pubkeysfile {
			newReposContent = append(newReposContent, repoContent{Type: trustType, KeyType: "GPGKeys", KeyPath: filepath})
		}

	case "sigstoreSigned":
		if len(pubkeysfile) == 0 {
			return errors.New("at least one public key must be defined for type 'sigstoreSigned'")
		}
		for _, filepath := range pubkeysfile {
			newReposContent = append(newReposContent, repoContent{Type: trustType, KeyPath: filepath})
		}

	default:
		return fmt.Errorf("unknown trust type %q", input.Type)
	}
	newReposJSON, err := json.Marshal(newReposContent)
	if err != nil {
		return err
	}

	err = fileutils.Exists(policyPath)
	if !errors.Is(err, os.ErrNotExist) {
		policyContent, err := os.ReadFile(policyPath)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(policyContent, &policyContentStruct); err != nil {
			return errors.New("could not read trust policies")
		}
	}
	if input.Scope == "default" {
		policyContentStruct.Default = json.RawMessage(newReposJSON)
	} else {
		if len(policyContentStruct.Default) == 0 {
			return errors.New("default trust policy must be set")
		}
		registryExists := false
		for transport, transportval := range policyContentStruct.Transports {
			_, registryExists = transportval[input.Scope]
			if registryExists {
				policyContentStruct.Transports[transport][input.Scope] = json.RawMessage(newReposJSON)
				break
			}
		}
		if !registryExists {
			if policyContentStruct.Transports == nil {
				policyContentStruct.Transports = make(map[string]genericRepoMap)
			}
			if policyContentStruct.Transports["docker"] == nil {
				policyContentStruct.Transports["docker"] = make(map[string]json.RawMessage)
			}
			policyContentStruct.Transports["docker"][input.Scope] = json.RawMessage(newReposJSON)
		}
	}

	data, err := json.MarshalIndent(policyContentStruct, "", "    ")
	if err != nil {
		return fmt.Errorf("setting trust policy: %w", err)
	}
	return os.WriteFile(policyPath, data, 0o644)
}

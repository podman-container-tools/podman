/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package pkcs11config

import (
	"errors"
	"fmt"
	"os"
	"path"

	"github.com/containers/ocicrypt/crypto/pkcs11"
	"go.yaml.in/yaml/v3"
)

// OcicryptConfig represents the format of an imgcrypt.conf config file
type OcicryptConfig struct {
	Pkcs11Config pkcs11.Pkcs11Config `yaml:"pkcs11"`
}

const CONFIGFILE = "ocicrypt.conf"
const ENVVARNAME = "OCICRYPT_CONFIG"

// parseConfigFile parses a configuration file; it is not an error if the configuration file does
// not exist, so no error is returned.
// A config file may look like this:
// module-directories:
// - /usr/lib64/pkcs11/
// - /usr/lib/pkcs11/
// allowed-module-paths:
// - /usr/lib64/pkcs11/
// - /usr/lib/pkcs11/
func parseConfigFile(filename string) (*OcicryptConfig, error) {
	// a non-existent config file is not an error
	_, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return nil, nil
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	ic := &OcicryptConfig{}
	err = yaml.Unmarshal(data, ic)
	return ic, err
}

// getConfiguration tries to read the configuration file at the following locations
// 1) ${OCICRYPT_CONFIG} == "internal": use internal default allow-all policy
// 2) ${OCICRYPT_CONFIG}
// 3) ${XDG_CONFIG_HOME}/ocicrypt-pkcs11.conf
// 4) ${HOME}/.config/ocicrypt-pkcs11.conf
// 5) /etc/ocicrypt-pkcs11.conf
// If no configuration file could be found or read a null pointer is returned
func getConfiguration() (*OcicryptConfig, error) {
	filename := os.Getenv(ENVVARNAME)
	if len(filename) > 0 {
		if filename == "internal" {
			return getDefaultCryptoConfigOpts()
		}
		ic, err := parseConfigFile(filename)
		if err != nil || ic != nil {
			return ic, err
		}
	}
	envvar := os.Getenv("XDG_CONFIG_HOME")
	if len(envvar) > 0 {
		ic, err := parseConfigFile(path.Join(envvar, CONFIGFILE))
		if err != nil || ic != nil {
			return ic, err
		}
	}
	envvar = os.Getenv("HOME")
	if len(envvar) > 0 {
		ic, err := parseConfigFile(path.Join(envvar, ".config", CONFIGFILE))
		if err != nil || ic != nil {
			return ic, err
		}
	}
	return parseConfigFile(path.Join("etc", CONFIGFILE))
}

// getDefaultCryptoConfigOpts returns default crypto config opts needed for pkcs11 module access
func getDefaultCryptoConfigOpts() (*OcicryptConfig, error) {
	mdyaml := pkcs11.GetDefaultModuleDirectoriesYaml("")
	config := fmt.Sprintf("module-directories:\n"+
		"%s"+
		"allowed-module-paths:\n"+
		"%s", mdyaml, mdyaml)
	p11conf, err := pkcs11.ParsePkcs11ConfigFile([]byte(config))
	return &OcicryptConfig{
		Pkcs11Config: *p11conf,
	}, err
}

// GetUserPkcs11Config gets the user's Pkcs11Conig either from a configuration file or if none is
// found the default ones are returned
func GetUserPkcs11Config() (*pkcs11.Pkcs11Config, error) {
	fmt.Print("Note: pkcs11 support is currently experimental\n")
	ic, err := getConfiguration()
	if err != nil {
		return &pkcs11.Pkcs11Config{}, err
	}
	if ic == nil {
		return &pkcs11.Pkcs11Config{}, errors.New("No ocicrypt config file was found")
	}
	return &ic.Pkcs11Config, nil
}

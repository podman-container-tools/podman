package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
)

// ReadPassphraseFile returns the first line of the specified path.
// For convenience, an empty string is returned if the path is empty.
func ReadPassphraseFile(path string) (string, error) {
	if path == "" {
		return "", nil
	}

	logrus.Debugf("Reading user-specified passphrase for signing from %s", path)

	ppf, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer ppf.Close()

	// Read the *first* line in the passphrase file, just as gpg(1) does.
	buf, err := bufio.NewReader(ppf).ReadBytes('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("reading passphrase file: %w", err)
	}

	return strings.TrimSuffix(string(buf), "\n"), nil
}

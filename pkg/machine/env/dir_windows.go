package env

import "os"

func getRuntimeDir() (string, error) {
	tmpDir, ok := os.LookupEnv("TEMP")
	if !ok {
		tmpDir = os.Getenv("LOCALAPPDATA") + "\\Temp"
	}
	return tmpDir, nil
}

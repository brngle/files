package files

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
)

func generateHash(volume *Volume, path string) (string, error) {
	f, err := volume.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	_, err = io.Copy(h, f)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

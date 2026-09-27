package core

import "os"

func writeSecret(path string, plain []byte) error {
	b, err := seal(plain)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0600)
}

func readSecret(path string) ([]byte, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return openSeal(b)
}

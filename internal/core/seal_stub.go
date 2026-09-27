//go:build !windows

package core

func seal(plain []byte) ([]byte, error) { return plain, nil }

func openSeal(b []byte) ([]byte, error) { return b, nil }

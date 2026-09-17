//go:build !linux && !darwin

package config

import (
	"errors"
	"os"
)

func tryLock(*os.File) (bool, error) {
	return false, errors.New("state-changing commands are supported on Linux and macOS only")
}

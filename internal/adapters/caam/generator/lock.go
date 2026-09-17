package generator

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"portal-static/internal/contracts"
)

var ErrBusy = contracts.ErrBusy

type fileLock struct {
	path string
	file *os.File
}

func acquireFileLock(path string, staleAfter time.Duration) (*fileLock, error) {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err == nil {
		_, _ = fmt.Fprintf(file, "pid=%d\ncreated_at=%s\n", os.Getpid(), time.Now().Format(time.RFC3339))
		return &fileLock{path: path, file: file}, nil
	}
	if !os.IsExist(err) {
		return nil, fmt.Errorf("create generation lock: %w", err)
	}
	info, statErr := os.Stat(path)
	if statErr == nil && staleAfter > 0 && time.Since(info.ModTime()) > staleAfter {
		if removeErr := os.Remove(path); removeErr == nil {
			return acquireFileLock(path, staleAfter)
		}
	}
	return nil, ErrBusy
}

func (l *fileLock) release() {
	if l == nil {
		return
	}
	if l.file != nil {
		_ = l.file.Close()
	}
	_ = os.Remove(l.path)
}

func (l *fileLock) String() string {
	return l.path + ":" + strconv.Itoa(os.Getpid())
}

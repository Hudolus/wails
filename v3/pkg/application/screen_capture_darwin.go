//go:build darwin

package application

import "fmt"

// getSourcesImpl returns an error on non-Windows platforms.
func getSourcesImpl() ([]*DesktopSource, []*DesktopSource, error) {
	return nil, nil, fmt.Errorf("desktop capture is not supported on this platform")
}

// captureImpl returns an error on non-Windows platforms.
func captureImpl(sourceType SourceType, id uintptr) (string, error) {
	return "", fmt.Errorf("desktop capture is not supported on this platform")
}

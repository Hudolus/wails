//go:build windows

package application

import (
	"fmt"
	"github.com/wailsapp/wails/v3/pkg/w32"
)

// getSourcesImpl is the Windows implementation for GetSources.
func getSourcesImpl() ([]*CaptureSource, []*CaptureSource, error) {
	sources, err := w32.GetDisplaySources()
	if err != nil {
		return nil, nil, err
	}

	var screens []*CaptureSource
	var windows []*CaptureSource

	for _, source := range sources {
		s := &CaptureSource{
			ID:   source.ID,
			Name: source.Name,
			Type: SourceType(source.Type),
		}
		if s.Type == STScreen {
			screens = append(screens, s)
		} else {
			windows = append(windows, s)
		}
	}

	return screens, windows, nil
}

// captureImpl is the Windows implementation for Capture.
func captureImpl(sourceType SourceType, id uintptr) (string, error) {
	// We need to find the original w32.DisplaySource to get the Rect info
	sources, err := w32.GetDisplaySources()
	if err != nil {
		return "", err
	}

	var targetSource w32.DisplaySource
	found := false
	for _, source := range sources {
		if SourceType(source.Type) == sourceType && source.ID == id {
			targetSource = source
			found = true
			break
		}
	}

	if !found {
		return "", fmt.Errorf("source with type '%s' and id '%d' not found", sourceType, id)
	}

	return w32.CaptureSource(targetSource)
}

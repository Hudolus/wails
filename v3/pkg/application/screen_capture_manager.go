package application

// SourceType defines whether the source is a screen or a window.
type SourceType string

const (
	STScreen SourceType = "screen"
	STWindow SourceType = "window"
)

// CaptureSource represents a single capturable source.
type CaptureSource struct {
	// The platform-specific ID of the source.
	ID uintptr `json:"id"`
	// The name of the source, e.g., "Display 1" or "Google Chrome".
	Name string `json:"name"`
	// The type of the source. Either "screen" or "window".
	Type SourceType `json:"type"`
}

// CaptureManager provides methods to capture the screen or individual windows.
type CaptureManager struct {
	app *App
}

// newCaptureManager creates a new CaptureManager.
func newCaptureManager(app *App) *CaptureManager {
	return &CaptureManager{
		app: app,
	}
}

// GetSources returns a list of all available screens and windows that can be captured.
// It calls the platform-specific implementation.
func (d *CaptureManager) GetSources() ([]*CaptureSource, []*CaptureSource, error) {
	return getSourcesImpl()
}

// Capture takes a source type and ID and returns a base64 encoded PNG of the capture.
// It calls the platform-specific implementation.
func (d *CaptureManager) Capture(sourceType SourceType, id uintptr) (string, error) {
	return captureImpl(sourceType, id)
}

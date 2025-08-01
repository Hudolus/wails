//go:build windows

package w32

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	// User32.dll procedures
	procGetMonitorInfoW      = moduser32.NewProc("GetMonitorInfoW")
	procIsIconic             = moduser32.NewProc("IsIconic")
	procGetWindowTextLengthW = moduser32.NewProc("GetWindowTextLengthW")
	procGetWindowTextW       = moduser32.NewProc("GetWindowTextW")
	procGetWindowLongPtrW    = moduser32.NewProc("GetWindowLongPtrW")
	procGetClassNameW        = moduser32.NewProc("GetClassNameW")
	procPrintWindow          = moduser32.NewProc("PrintWindow")

	// Gdi32.dll procedures
	procCreateCompatibleBitmap = modgdi32.NewProc("CreateCompatibleBitmap")
	procGetDIBits              = modgdi32.NewProc("GetDIBits")
)

type SourceType string

const (
	STScreen SourceType = "screen"
	STWindow SourceType = "window"

	WsExToolwindow      = 0x00000080
	GwOwner             = 4
	DWMWACloaked        = 14
	SRCCopy             = 0x00CC0020
	CaptureBlt          = 0x40000000
	DibRgbColors        = 0
	BiRgb               = 0
	CCHDeviceName       = 32
	PwRenderFullContent = 0x00000002
	GMemMoveable        = 0x0002
)

// MonitorInfo contains information about a display monitor.
type MonitorInfo struct {
	CbSize    uint32
	RcMonitor windows.Rect
	RcWork    windows.Rect
	DwFlags   uint32
}

// MonitorInfoEx is an extended version of MonitorInfo.
type MonitorInfoEx struct {
	CbSize     uint32
	RcMonitor  windows.Rect
	RcWork     windows.Rect
	DwFlags    uint32
	DeviceName [CCHDeviceName]uint16
}

// DisplaySource holds information about a capturable source.
type DisplaySource struct {
	ID   uintptr
	Name string
	Type SourceType
	Rect windows.Rect
}

// BitmapInfoHeader defines the structure for bitmap information.
type BitmapInfoHeader struct {
	BiSize          uint32
	BiWidth         int32
	BiHeight        int32
	BiPlanes        uint16
	BiBitCount      uint16
	BiCompression   uint32
	BiSizeImage     uint32
	BiXPelsPerMeter int32
	BiYPelsPerMeter int32
	BiClrUsed       uint32
	BiClrImportant  uint32
}

func GetDisplaySources() ([]DisplaySource, error) {
	var sources []DisplaySource
	var monitorCount int

	monitorCallback := func(hMonitor uintptr, hdc uintptr, rect *windows.Rect, lparam uintptr) uintptr {
		info := MonitorInfoEx{}
		info.CbSize = uint32(unsafe.Sizeof(info))
		ret, _, _ := procGetMonitorInfoW.Call(hMonitor, uintptr(unsafe.Pointer(&info)))
		if ret == 0 {
			return 1
		}
		source := DisplaySource{
			ID:   hMonitor,
			Name: fmt.Sprintf("Display %d", monitorCount),
			Type: STScreen,
			Rect: info.RcMonitor,
		}
		sources = append(sources, source)
		return 1
	}
	ret, _, err := procEnumDisplayMonitors.Call(0, 0, syscall.NewCallback(monitorCallback), 0)
	if ret == 0 {
		return nil, fmt.Errorf("EnumDisplayMonitors failed: %w", err)
	}

	windowCallback := func(hwnd uintptr, lparam uintptr) uintptr {
		if visible, _, _ := procIsWindowVisible.Call(hwnd); visible == 0 {
			return 1
		}
		if length, _, _ := procGetWindowTextLengthW.Call(hwnd); length == 0 {
			return 1
		}
		if owner, _, _ := procGetWindow.Call(hwnd, GwOwner); owner != 0 {
			return 1
		}

		var r windows.Rect
		_, _, err = procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
		if err != nil {
			return 0
		}
		if r.Right-r.Left <= 0 || r.Bottom-r.Top <= 0 {
			return 1
		}

		if minimized, _, _ := procIsIconic.Call(hwnd); minimized != 0 {
			return 1
		}
		if style, _, _ := procGetWindowLongPtrW.Call(hwnd, ^uintptr(19)); (style & WsExToolwindow) != 0 {
			return 1
		}

		var isCloaked uint32
		_, _, err = procDwmGetWindowAttribute.Call(hwnd, DWMWACloaked, uintptr(unsafe.Pointer(&isCloaked)), unsafe.Sizeof(isCloaked))
		if err != nil {
			return 0
		}
		if isCloaked != 0 {
			return 1
		}

		classNameBuffer := make([]uint16, 256)
		_, _, err = procGetClassNameW.Call(hwnd, uintptr(unsafe.Pointer(&classNameBuffer[0])), 256)
		if err != nil {
			return 0
		}

		className := syscall.UTF16ToString(classNameBuffer)
		if className == "Windows.UI.Core.CoreWindow" || className == "CiceroUIWndFrame" || className == "ApplicationFrameWindow" {
			return 1
		}
		textLen, _, _ := procGetWindowTextLengthW.Call(hwnd)
		buffer := make([]uint16, textLen+1)
		_, _, err = procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buffer[0])), textLen+1)
		if err != nil {
			return 0
		}
		title := syscall.UTF16ToString(buffer)
		source := DisplaySource{
			ID:   hwnd,
			Name: title,
			Type: STWindow,
			Rect: r,
		}
		sources = append(sources, source)
		return 1
	}
	ret, _, err = procEnumWindows.Call(syscall.NewCallback(windowCallback), 0)
	if ret == 0 {
		return nil, fmt.Errorf("EnumWindows failed: %w", err)
	}

	return sources, nil
}

func CaptureSource(source DisplaySource) (string, error) {
	r := source.Rect
	width := r.Right - r.Left
	height := r.Bottom - r.Top

	screenDC, _, _ := procGetDC.Call(0)
	if screenDC == 0 {
		return "", fmt.Errorf("GetDC(0) failed")
	}
	defer procReleaseDC.Call(0, screenDC)

	memDC, _, _ := procCreateCompatibleDC.Call(screenDC)
	if memDC == 0 {
		return "", fmt.Errorf("CreateCompatibleDC failed")
	}
	defer procDeleteDC.Call(memDC)

	bitmap, _, _ := procCreateCompatibleBitmap.Call(screenDC, uintptr(width), uintptr(height))
	if bitmap == 0 {
		return "", fmt.Errorf("CreateCompatibleBitmap failed")
	}
	defer procDeleteObject.Call(bitmap)

	oldBitmap, _, _ := procSelectObject.Call(memDC, bitmap)
	if oldBitmap == 0 {
		return "", fmt.Errorf("SelectObject failed")
	}
	defer procSelectObject.Call(memDC, oldBitmap)

	if source.Type == STWindow {
		ret, _, _ := procPrintWindow.Call(source.ID, memDC, PwRenderFullContent)
		if ret == 0 {
			ret, _, _ = procBitBlt.Call(memDC, 0, 0, uintptr(width), uintptr(height), screenDC, uintptr(r.Left), uintptr(r.Top), SRCCopy|CaptureBlt)
			if ret == 0 {
				return "", fmt.Errorf("PrintWindow and BitBlt failed for window")
			}
		}
	} else {
		ret, _, _ := procBitBlt.Call(memDC, 0, 0, uintptr(width), uintptr(height), screenDC, uintptr(r.Left), uintptr(r.Top), SRCCopy)
		if ret == 0 {
			return "", fmt.Errorf("BitBlt failed for screen")
		}
	}

	header := BitmapInfoHeader{
		BiSize:        uint32(unsafe.Sizeof(BitmapInfoHeader{})),
		BiWidth:       width,
		BiHeight:      -height,
		BiPlanes:      1,
		BiBitCount:    32,
		BiCompression: BiRgb,
	}

	// Allocate memory for the GetDIBits buffer using the Windows API.
	bitmapDataSize := uintptr(width * height * 4)
	hmem, _, err := procGlobalAlloc.Call(GMemMoveable, bitmapDataSize)
	if hmem == 0 {
		return "", fmt.Errorf("GlobalAlloc failed: %w", err)
	}
	defer procGlobalFree.Call(hmem)

	memptr, _, err := procGlobalLock.Call(hmem)
	if memptr == 0 {
		return "", fmt.Errorf("GlobalLock failed: %w", err)
	}
	defer procGlobalUnlock.Call(hmem)

	// Call GetDIBits with the pointer to the globally allocated memory.
	ret, _, err := procGetDIBits.Call(memDC, bitmap, 0, uintptr(height), memptr, uintptr(unsafe.Pointer(&header)), DibRgbColors)
	if ret == 0 {
		return "", fmt.Errorf("GetDIBits failed: %w", err)
	}

	// Create an image.RGBA and manually copy the pixel data from the Windows memory buffer.
	img := image.NewRGBA(image.Rect(0, 0, int(width), int(height)))
	src := memptr
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i] = *(*byte)(unsafe.Pointer(src + 2))   // B
		img.Pix[i+1] = *(*byte)(unsafe.Pointer(src + 1)) // G
		img.Pix[i+2] = *(*byte)(unsafe.Pointer(src))     // R
		img.Pix[i+3] = 255                               // A
		src += 4
	}

	var buf bytes.Buffer
	if err = png.Encode(&buf, img); err != nil {
		return "", fmt.Errorf("png.Encode failed: %w", err)
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

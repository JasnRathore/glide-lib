// +build windows
package glide

import (
	"fmt"
	"log"
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"github.com/jchv/go-webview2"
	utils "github.com/JasnRathore/glide-lib/utils"
)

// MARGINS structure for DwmExtendFrameIntoClientArea
type MARGINS struct {
	Left, Right, Top, Bottom int32
}

type App struct {
	webview webview2.WebView
	wg      sync.WaitGroup
	quit    chan struct{}
	tray    *trayManager
	config  AppConfig
}

var (
	user32           = syscall.NewLazyDLL("user32.dll")
	showWindow       = user32.NewProc("ShowWindow")
	showWindowAsync  = user32.NewProc("ShowWindowAsync")
	setWindowPos     = user32.NewProc("SetWindowPos")
	getSystemMetrics = user32.NewProc("GetSystemMetrics")
	setLayeredWindowAttributes = user32.NewProc("SetLayeredWindowAttributes")
	dwmExtendFrameIntoClientArea = syscall.NewLazyDLL("dwmapi.dll").NewProc("DwmExtendFrameIntoClientArea")
	getDC = user32.NewProc("GetDC")
	releaseDC = user32.NewProc("ReleaseDC")
)

// Window constants
const (
	SW_HIDE     = 0
	SW_SHOW     = 5
	SW_MINIMIZE = 6
	SW_MAXIMIZE = 3
	SW_RESTORE  = 9
	
	// Window styles
	WS_CAPTION     = 0x00C00000
	WS_THICKFRAME  = 0x00040000
	WS_MINIMIZEBOX = 0x00020000
	WS_MAXIMIZEBOX = 0x00010000
	WS_SYSMENU     = 0x00080000
	WS_BORDER      = 0x00800000

	// SetWindowPos flags
	SWP_NOSIZE         = 0x0001
	SWP_NOZORDER       = 0x0004
	SWP_NOACTIVATE     = 0x0010
	SWP_SHOWWINDOW     = 0x0040
	SWP_FRAMECHANGED   = 0x0020
	SWP_NOOWNERZORDER  = 0x0200
	
	// GetSystemMetrics constants
	SM_CXSCREEN        = 0
	SM_CYSCREEN        = 1
	SM_CXVIRTUALSCREEN = 78
	SM_CYVIRTUALSCREEN = 79
	SM_XVIRTUALSCREEN  = 76
	SM_YVIRTUALSCREEN  = 77
	
	// Window transparency constants
	WS_EX_LAYERED      = 0x00080000
	LWA_ALPHA          = 0x00000002
	LWA_COLORKEY       = 0x00000001
	
	// Constants for transparency
	GWL_STYLE          = -16
	GWL_EXSTYLE        = -20
	WS_EX_TRANSPARENT  = 0x00000020
	WS_EX_COMPOSITED   = 0x02000000
	WS_EX_APPWINDOW    = 0x00040000
)

// Using functions instead of constants to avoid the uintptr overflow issue
func gwlStyle() int32 {
	return -16
}

func gwlExStyle() int32 {
	return -20
}

// Get appropriate window long proc based on architecture
func getWindowLongProc() *syscall.LazyProc {
	if unsafe.Sizeof(uintptr(0)) == 8 {
		return user32.NewProc("GetWindowLongPtrW")
	}
	return user32.NewProc("GetWindowLongW")
}

// Set appropriate window long proc based on architecture
func setWindowLongProc() *syscall.LazyProc {
	if unsafe.Sizeof(uintptr(0)) == 8 {
		return user32.NewProc("SetWindowLongPtrW")
	}
	return user32.NewProc("SetWindowLongW")
}

// SetTransparency sets the window transparency level (0-255, where 0 is fully transparent and 255 is fully opaque)
func (a *App) SetTransparency(alpha byte) {
	if a.webview == nil {
		log.Println("Webview not initialized, cannot set transparency")
		return
	}

	hwnd := a.webview.Window()
	
	a.webview.Dispatch(func() {
		// Get current window style
		exStyle, _, _ := getWindowLongProc().Call(
			uintptr(hwnd),
			uintptr(gwlExStyle()),
		)
		
		// Add layered window style
		newExStyle := exStyle | WS_EX_LAYERED
		setWindowLongProc().Call(
			uintptr(hwnd),
			uintptr(gwlExStyle()),
			newExStyle,
		)
		
		// Set the transparency level
		setLayeredWindowAttributes.Call(
			uintptr(hwnd),
			0,                  // crKey - color to make transparent, 0 means not used
			uintptr(alpha),     // alpha value (0-255)
			uintptr(LWA_ALPHA), // use alpha value
		)
	})
}

// SetBackgroundTransparent makes the window background transparent while keeping the content visible
// SetBackgroundTransparent makes the window background transparent while keeping the content visible
func (a *App) SetBackgroundTransparent() error {
    if a.webview == nil {
        return fmt.Errorf("Webview not initialized, cannot set transparent background")
    }

    hwnd := a.webview.Window()
    
    a.webview.Dispatch(func() {
        // Get current extended window style
        exStyle, _, _ := getWindowLongProc().Call(
            uintptr(hwnd),
            uintptr(gwlExStyle()),
        )
        
        // Add necessary styles for transparency
        newExStyle := uintptr(WS_EX_LAYERED | WS_EX_TRANSPARENT | WS_EX_COMPOSITED)
        setWindowLongProc().Call(
            uintptr(hwnd),
            uintptr(gwlExStyle()),
            newExStyle,
        )
        
        // Make the window transparent (0 alpha)
        setLayeredWindowAttributes.Call(
            uintptr(hwnd),
            0,                  // Color key (not used)
            0,                  // Alpha (0 = fully transparent)
            uintptr(LWA_ALPHA), // Use alpha channel
        )
        
        // Remove the background brush
        const GWL_HBRBACKGROUND = -10
        setWindowLongProc().Call(
            uintptr(hwnd),
            uintptr(GWL_HBRBACKGROUND),
            uintptr(0), // NULL brush
        )
        
        // Force redraw
        showWindow.Call(
            uintptr(hwnd),
            uintptr(SW_SHOW),
        )
        
        // Update the window
        updateWindow := user32.NewProc("UpdateWindow")
        updateWindow.Call(uintptr(hwnd))
    })
    
    return nil
}	

// ScreenSize represents the dimensions of a screen/monitor
type ScreenSize struct {
	Width  int
	Height int
}

// VirtualScreenInfo represents the virtual screen dimensions and position
type VirtualScreenInfo struct {
	Width  int
	Height int
	X      int
	Y      int
}

// GetScreenSize returns the primary monitor resolution
func (a *App) GetScreenSize() ScreenSize {
	width, _, _ := getSystemMetrics.Call(SM_CXSCREEN)
	height, _, _ := getSystemMetrics.Call(SM_CYSCREEN)
	
	return ScreenSize{
		Width:  int(width),
		Height: int(height),
	}
}

// GetVirtualScreenInfo returns information about the virtual screen
// Virtual screen encompasses all display monitors
func (a *App) GetVirtualScreenInfo() VirtualScreenInfo {
	width, _, _ := getSystemMetrics.Call(SM_CXVIRTUALSCREEN)
	height, _, _ := getSystemMetrics.Call(SM_CYVIRTUALSCREEN)
	x, _, _ := getSystemMetrics.Call(SM_XVIRTUALSCREEN)
	y, _, _ := getSystemMetrics.Call(SM_YVIRTUALSCREEN)
	
	return VirtualScreenInfo{
		Width:  int(width),
		Height: int(height),
		X:      int(x),
		Y:      int(y),
	}
}

// SetPosition sets the window position to the specified x and y coordinates
func (a *App) SetPosition(x, y int) {
	if a.webview == nil {
		log.Println("Webview not initialized, cannot set position")
		return
	}

	hwnd := a.webview.Window()
	a.webview.Dispatch(func() {
		// SetWindowPos parameters:
		// hWnd, hWndInsertAfter, X, Y, cx, cy, uFlags
		// SWP_NOSIZE | SWP_NOZORDER means don't change size or z-order, just position
		setWindowPos.Call(
			uintptr(hwnd),
			0,                // hWndInsertAfter = 0 (no z-order change)
			uintptr(x),       // x position
			uintptr(y),       // y position
			0,                // width (ignored with SWP_NOSIZE)
			0,                // height (ignored with SWP_NOSIZE)
			uintptr(SWP_NOSIZE|SWP_NOZORDER), // don't change size or z-order
		)
	})
}

func (a *App) RemoveBorders() {
	if a.webview == nil {
		return
	}

	hwnd := a.webview.Window()
	
	// Get current window style
	style, _, _ := getWindowLongProc().Call(
		uintptr(hwnd),
		uintptr(gwlStyle()),
	)
	
	// Remove caption and border styles
	newStyle := style &^ uintptr(WS_CAPTION|WS_THICKFRAME|WS_MINIMIZEBOX|WS_MAXIMIZEBOX|WS_SYSMENU|WS_BORDER)
	
	// Set new window style
	setWindowLongProc().Call(
		uintptr(hwnd),
		uintptr(gwlStyle()),
		newStyle,
	)
	
	// Force redraw
	a.webview.Dispatch(func() {
		showWindow.Call(
			uintptr(hwnd),
			uintptr(SW_SHOW),
		)
	})
}

// RestoreBorders restores the default window borders and title bar
func (a *App) RestoreBorders() {
	if a.webview == nil {
		return
	}

	hwnd := a.webview.Window()
	
	// Get default window style
	style, _, _ := getWindowLongProc().Call(
		uintptr(hwnd),
		uintptr(gwlStyle()),
	)
	
	// Add back caption and border styles
	newStyle := style | uintptr(WS_CAPTION|WS_THICKFRAME|WS_MINIMIZEBOX|WS_MAXIMIZEBOX|WS_SYSMENU|WS_BORDER)
	
	// Set new window style
	setWindowLongProc().Call(
		uintptr(hwnd),
		uintptr(gwlStyle()),
		newStyle,
	)
	
	// Force redraw
	a.webview.Dispatch(func() {
		showWindow.Call(
			uintptr(hwnd),
			uintptr(SW_SHOW),
		)
	})
}

func hideWindow(w webview2.WebView) {
	hwnd := w.Window()
	showWindow.Call(uintptr(hwnd), SW_HIDE)
}

func myShowWindow(w webview2.WebView) {
	hwnd := w.Window()
	showWindow.Call(uintptr(hwnd), SW_SHOW)
}

// Maximize maximizes the application window
func (a *App) Maximize() {
	if a.webview != nil {
		hwnd := a.webview.Window()
		showWindowAsync.Call(uintptr(hwnd), SW_MAXIMIZE)
	}
}

// Minimize minimizes the application window
func (a *App) Minimize() {
	if a.webview != nil {
		hwnd := a.webview.Window()
		showWindowAsync.Call(uintptr(hwnd), SW_MINIMIZE)
	}
}

// Restore restores the window from minimized or maximized state
func (a *App) Restore() {
	if a.webview != nil {
		hwnd := a.webview.Window()
		showWindowAsync.Call(uintptr(hwnd), SW_RESTORE)
	}
}

func (a *App) ShowWindow() {
    if a.webview != nil {
        myShowWindow(a.webview)
    }
}

func (a *App) Exit() {
    a.Terminate()
}

func New(config AppConfig) *App {
	app := &App{
		quit:   make(chan struct{}),
		config: config,
	}

	if config.Tray != nil {
		app.tray = newTrayManager(config.Tray, app)
	}

	app.initializeWebview()
	return app
}

func (a *App) initializeWebview() {
	w := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug:     a.config.Debug,
		AutoFocus: a.config.AutoFocus,
		WindowOptions: webview2.WindowOptions{
			Title:  a.config.Title,
			Width:  a.config.Width,
			Height: a.config.Height,
			Center: a.config.Center,
			IconId: uint(a.config.IconID),
		},
	})

	if w == nil {
		log.Fatalln("Failed to load webview.")
	}
	a.webview = w
}

func (a *App) Run() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if a.tray != nil {
		a.wg.Add(1)
		go func() {
			defer a.wg.Done()
			a.tray.run()
		}()
	}

	a.webview.Run()
	a.wg.Wait()
}

func (a *App) Terminate() {
	close(a.quit)
	if a.webview != nil {
		a.webview.Terminate()
	}
}

func (a *App) Navigate(url string) {
	if a.webview == nil {
		log.Println("Webview not initialized, cannot navigate")
		return
	}
	a.webview.Dispatch(func() {
		a.webview.Navigate(url)
	})
}

func (a *App) RunWithURL(url string) {
	a.Navigate(url)
	a.Run()
}

func (a *App) InvokeHandler(funcs []interface{}) {
	for _, fn := range funcs {
		name := utils.FuncToString(fn)
		a.webview.Bind(name,fn)
	}
}

func (a *App) AddMenuItem(item MenuItem) {
	if a.tray != nil {
		a.tray.AddMenuItem(item)
	}
}

func (a *App) GetWebView() webview2.WebView {
	return a.webview
}
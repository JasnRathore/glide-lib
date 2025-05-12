// +build windows
package glide

import (
	"log"
	"runtime"
	"sync"
	"syscall"

	"github.com/jchv/go-webview2"
	utils "github.com/JasnRathore/glide-lib/utils"
)

type App struct {
	webview webview2.WebView
	wg      sync.WaitGroup
	quit    chan struct{}
	tray    *trayManager
	config  AppConfig
}

var (
	user32     = syscall.NewLazyDLL("user32.dll")
	showWindow = user32.NewProc("ShowWindow")
	showWindowAsync  = user32.NewProc("ShowWindowAsync")
)

const (
	SW_HIDE = 0
	SW_SHOW = 5
	SW_MINIMIZE = 6
	SW_MAXIMIZE = 3
	SW_RESTORE = 9
)

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
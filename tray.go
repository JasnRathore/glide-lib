// +build windows

package glide

import (
	"sync"
	"github.com/getlantern/systray"
)

type trayManager struct {
	config        *TrayConfig
	app           *App
	quit          chan struct{}
	menuItems     []*systray.MenuItem
	itemMutex     sync.Mutex
	pendingItems  []MenuItem
	initialized   bool
	initializedCh chan struct{}
}

func newTrayManager(config *TrayConfig, app *App) *trayManager {
	return &trayManager{
		config:       config,
		app:          app,
		quit:         app.quit,
		menuItems:    make([]*systray.MenuItem, 0),
		pendingItems: make([]MenuItem, 0),
		initialized:  false,
		initializedCh: make(chan struct{}),
	}
}

func (t *trayManager) run() {
	go systray.Run(t.onReady, t.onExit)
	<-t.initializedCh // Wait for initialization to complete
}

func (t *trayManager) onReady() {
	t.itemMutex.Lock()
	defer t.itemMutex.Unlock()

	// Set initial properties
	if t.config.Title != "" {
		systray.SetTitle(t.config.Title)
	}
	if t.config.Tooltip != "" {
		systray.SetTooltip(t.config.Tooltip)
	}

	// Process config menu items
	t.buildMenu(t.config.MenuItems)

	// Process any pending items added before initialization
	for _, item := range t.pendingItems {
		t.addMenuItem(item)
	}
	t.pendingItems = nil

	// Mark as initialized
	t.initialized = true
	close(t.initializedCh)

	if t.config.OnReady != nil {
		t.config.OnReady()
	}
}

func (t *trayManager) buildMenu(items []MenuItem) {
	for _, item := range items {
		t.AddMenuItem(item)
	}
}

func (t *trayManager) AddMenuItem(item MenuItem) {
	t.itemMutex.Lock()
	defer t.itemMutex.Unlock()

	if t.initialized {
		t.addMenuItem(item)
	} else {
		t.pendingItems = append(t.pendingItems, item)
	}
}

func (t *trayManager) addMenuItem(item MenuItem) {
	m := systray.AddMenuItem(item.Title, item.Tooltip)
	t.menuItems = append(t.menuItems, m)

	if item.Disabled {
		m.Disable()
	}

	if item.Checked {
		m.Check()
	}

	go func() {
		for range m.ClickedCh {
			if item.Handler != nil {
				item.Handler()
			}
		}
	}()

	for _, subItem := range item.Items {
		sub := m.AddSubMenuItem(subItem.Title, subItem.Tooltip)
		if subItem.Disabled {
			sub.Disable()
		}
		if subItem.Checked {
			sub.Check()
		}
		go func(si MenuItem) {
			for range sub.ClickedCh {
				if si.Handler != nil {
					si.Handler()
				}
			}
		}(subItem)
	}
}

func (t *trayManager) onExit() {
	if t.config.OnExit != nil {
		t.config.OnExit()
	}
	t.app.Terminate()
}
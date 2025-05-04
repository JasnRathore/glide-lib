package glide

type AppConfig struct {
	Debug     bool
	AutoFocus bool
	
	Title  string
	Width  uint
	Height uint
	Center bool
	IconID uint16
	
	Tray *TrayConfig
}

type TrayConfig struct {
	IconID    uint16
	Title     string
	Tooltip   string
	MenuItems []MenuItem
	OnReady   func()
	OnExit    func()
}

type MenuItem struct {
	Title    string
	Tooltip  string
	Disabled bool
	Checked  bool
	Handler  func()
	Items    []MenuItem
}
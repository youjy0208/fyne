// Package app provides app implementations for working with Fyne graphical interfaces.
// The fastest way to get started is to call app.New() which will normally load a new desktop application.
// If the "ci" tag is passed to go (go run -tags ci myapp.go) it will run an in-memory application.
package app // import "fyne.io/fyne/v2/app"

import (
	"os/exec"
	"strconv"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/internal"
	"fyne.io/fyne/v2/internal/app"
	intRepo "fyne.io/fyne/v2/internal/repository"
	"fyne.io/fyne/v2/storage/repository"
)

// Declare conformity with App interface
var _ fyne.App = (*fyneApp)(nil)

type fyneApp struct {
	driver   fyne.Driver
	icon     fyne.Resource
	uniqueID string

	cloud     fyne.CloudProvider
	lifecycle fyne.Lifecycle
	settings  *settings
	storage   *store
	prefs     fyne.Preferences

	running  bool
	runMutex sync.Mutex
	exec     func(name string, arg ...string) *exec.Cmd
}

func (a *fyneApp) CloudProvider() fyne.CloudProvider {
	return a.cloud
}

func (a *fyneApp) Icon() fyne.Resource {
	return a.icon
}

func (a *fyneApp) SetIcon(icon fyne.Resource) {
	a.icon = icon
}

func (a *fyneApp) UniqueID() string {
	if a.uniqueID != "" {
		return a.uniqueID
	}

	fyne.LogError("Preferences API requires a unique ID, use app.NewWithID()", nil)
	a.uniqueID = "missing-id-" + strconv.FormatInt(time.Now().Unix(), 10) // This is a fake unique - it just has to not be reused...
	return a.uniqueID
}

func (a *fyneApp) NewWindow(title string) fyne.Window {
	return a.driver.CreateWindow(title)
}

func (a *fyneApp) Run() {
	a.runMutex.Lock()

	if a.running {
		a.runMutex.Unlock()
		return
	}

	a.running = true
	a.runMutex.Unlock()

	a.driver.Run()
}

func (a *fyneApp) SetCloudProvider(p fyne.CloudProvider) {
	if p == nil {
		a.cloud = nil
		return
	}

	go a.transitionCloud(p)
}

func (a *fyneApp) transitionCloud(p fyne.CloudProvider) {
	err := p.Setup(a)
	if err != nil {
		fyne.LogError("Failed to set up cloud provider "+p.ProviderName(), err)
		return
	}
	a.cloud = p

	listeners := a.prefs.ChangeListeners()
	if pp, ok := p.(fyne.CloudProviderPreferences); ok {
		a.prefs = pp.CloudPreferences(a)
	} else {
		a.prefs = a.newDefaultPreferences()
	}

	for _, l := range listeners {
		a.prefs.AddChangeListener(l)
		l() // assume that preferences have changed because we replaced the provider
	}

	// after transition ensure settings listener is fired
	a.settings.apply()
}

func (a *fyneApp) Quit() {
	for _, window := range a.driver.AllWindows() {
		window.Close()
	}

	a.driver.Quit()
	a.settings.stopWatching()
	a.running = false
}

func (a *fyneApp) Driver() fyne.Driver {
	return a.driver
}

// Settings returns the application settings currently configured.
func (a *fyneApp) Settings() fyne.Settings {
	return a.settings
}

func (a *fyneApp) Storage() fyne.Storage {
	return a.storage
}

func (a *fyneApp) Preferences() fyne.Preferences {
	if a.uniqueID == "" {
		fyne.LogError("Preferences API requires a unique ID, use app.NewWithID()", nil)
	}
	return a.prefs
}

func (a *fyneApp) Lifecycle() fyne.Lifecycle {
	return a.lifecycle
}

func (a *fyneApp) newDefaultPreferences() fyne.Preferences {
	p := fyne.Preferences(newPreferences(a))
	if pref, ok := p.(interface{ load() }); ok && a.uniqueID != "" {
		pref.load()
	}
	return p
}

// New returns a new application instance with the default driver and no unique ID
func New() fyne.App {
	internal.LogHint("Applications should be created with a unique ID using app.NewWithID()")
	return NewWithID("")
}

func newAppWithDriver(d fyne.Driver, id string) fyne.App {
	newApp := &fyneApp{uniqueID: id, driver: d, exec: exec.Command, lifecycle: &app.Lifecycle{}}
	fyne.SetCurrentApp(newApp)

	newApp.prefs = newApp.newDefaultPreferences()
	newApp.settings = loadSettings()
	newApp.storage = &store{a: newApp}

	if !d.Device().IsMobile() {
		newApp.settings.watchSettings()
	}

	repository.Register("http", intRepo.NewHTTPRepository())
	repository.Register("https", intRepo.NewHTTPRepository())

	return newApp
}

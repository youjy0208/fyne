package binding

import (
	"sync"

	"fyne.io/fyne/v2"
)

type preferenceItem interface {
	checkForChange()
}

type preferenceBindings struct {
	lock  sync.RWMutex
	items map[string]preferenceItem
}

func (b *preferenceBindings) getItem(key string) preferenceItem {
	b.lock.RLock()
	item := b.items[key]
	b.lock.RUnlock()
	return item
}

func (b *preferenceBindings) list() []preferenceItem {
	b.lock.RLock()
	allItems := b.items
	b.lock.RUnlock()
	ret := make([]preferenceItem, 0, len(allItems))
	for _, i := range allItems {
		ret = append(ret, i)
	}
	return ret
}

func (b *preferenceBindings) setItem(key string, item preferenceItem) {
	b.lock.Lock()
	b.items[key] = item
	b.lock.Unlock()
}

type preferencesMap struct {
	lock  sync.RWMutex
	prefs map[fyne.Preferences]*preferenceBindings

	appPrefs fyne.Preferences // the main application prefs, to check if it changed...
}

func newPreferencesMap() *preferencesMap {
	return &preferencesMap{
		prefs: make(map[fyne.Preferences]*preferenceBindings),
	}
}

func (m *preferencesMap) ensurePreferencesAttached(p fyne.Preferences) *preferenceBindings {
	m.lock.RLock()
	binds := m.prefs[p]
	m.lock.RUnlock()

	if binds != nil {
		return binds
	}

	m.lock.Lock()
	m.prefs[p] = &preferenceBindings{
		items: make(map[string]preferenceItem),
	}
	binds = m.prefs[p]
	m.lock.Unlock()

	p.AddChangeListener(func() {
		m.preferencesChanged(fyne.CurrentApp().Preferences())
	})
	return binds
}

func (m *preferencesMap) getBindings(p fyne.Preferences) *preferenceBindings {
	m.lock.RLock()
	if p == fyne.CurrentApp().Preferences() {
		if m.appPrefs == nil {
			m.appPrefs = p
		} else if m.appPrefs != p {
			m.migratePreferences(m.appPrefs, p)
		}
	}

	binds := m.prefs[p]
	m.lock.RUnlock()
	return binds
}

func (m *preferencesMap) preferencesChanged(p fyne.Preferences) {
	binds := m.getBindings(p)
	if binds == nil {
		return
	}
	for _, item := range binds.list() {
		item.checkForChange()
	}
}

func (m *preferencesMap) migratePreferences(p1, p2 fyne.Preferences) {
	m.prefs[p2] = m.prefs[p1]
	delete(m.prefs, p1)
	m.appPrefs = p2

	for _, b := range m.prefs[p2].list() {
		if backed, ok := b.(interface{ replaceProvider(fyne.Preferences) }); ok {
			backed.replaceProvider(p2)
		}
	}

	m.preferencesChanged(p2)
}

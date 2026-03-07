package i18n

type Manager struct {
	translations map[string]map[string]string
}

func New() *Manager {
	return &Manager{
		translations: make(map[string]map[string]string),
	}
}

func (m *Manager) Add(lang string, key string, value string) {
	if _, ok := m.translations[lang]; !ok {
		m.translations[lang] = make(map[string]string)
	}
	m.translations[lang][key] = value
}

func (m *Manager) T(lang, key string) string {
	if v, ok := m.translations[lang][key]; ok {
		return v
	}
	return key
}

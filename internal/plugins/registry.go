package plugins

var registry = map[string]Factory{}

func Register(name string, factory Factory) {
	if name == "" || factory == nil {
		panic("plugins: invalid registration")
	}
	if _, exists := registry[name]; exists {
		panic("plugins: duplicate registration for " + name)
	}
	registry[name] = factory
}

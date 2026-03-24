package plugins

var registry = map[string]Factory{}

// Register makes a plugin factory available to Foundry by name.
//
// Registration is expected to happen from package init functions in plugin
// packages. Register panics for empty names, nil factories, or duplicate names
// because those are programmer errors that would make plugin loading
// ambiguous.
func Register(name string, factory Factory) {
	// TODO stop panicking and error gracefully
	if name == "" || factory == nil {
		panic("plugins: invalid registration")
	}
	if _, exists := registry[name]; exists {
		panic("plugins: duplicate registration for " + name)
	}
	registry[name] = factory
}

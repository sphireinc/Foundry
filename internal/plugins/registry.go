package plugins

var registry = map[string]Factory{}

func Register(name string, factory Factory) {
	registry[name] = factory
}

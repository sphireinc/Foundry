package cache

type Store interface {
	Get(key string) (any, bool)
	Set(key string, value any)
	Delete(key string)
	Clear()
}

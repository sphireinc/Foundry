package data

type Store struct {
	values map[string]any
}

func New() *Store {
	return &Store{
		values: make(map[string]any),
	}
}

func (s *Store) Set(key string, value any) {
	s.values[key] = value
}

func (s *Store) Get(key string) (any, bool) {
	v, ok := s.values[key]
	return v, ok
}

func (s *Store) All() map[string]any {
	return s.values
}

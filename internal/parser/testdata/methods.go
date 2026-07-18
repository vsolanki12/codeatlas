package cache

type Store struct{}

func (s Store) Get(key string) string {
	return ""
}

func (s Store) Size() int {
	return 0
}

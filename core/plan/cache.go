package plan

const (
	CacheTypeShared = "shared"
	CacheTypeLocked = "locked"
)

type Cache struct {
	Directory string `json:"directory,omitempty"`
	Type      string `json:"type,omitempty"`
}

func NewCache(directory string) *Cache {
	return &Cache{
		Directory: directory,
		Type:      CacheTypeShared,
	}
}

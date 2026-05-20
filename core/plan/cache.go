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

// BuildKitCacheMount declares a typed BuildKit `--mount=type=cache` directive
// scoped to a single Step. Unlike the legacy Caches map on BuildPlan (which
// is shared by name across steps), BuildKitCacheMount entries are inline and
// don't require a global registration.
//
// Renderer output:
//
//	RUN --mount=type=cache,target=<Target>,sharing=<Sharing> <cmd>
//
// Sharing defaults to "locked" when empty (BuildKit's safest default for
// package-manager caches that do their own locking).
type BuildKitCacheMount struct {
	Target  string `json:"target"`
	Sharing string `json:"sharing,omitempty"`
}

// NewBuildKitCacheMount creates a mount with the given target and the default
// "locked" sharing semantics. Use this from provider code rather than
// constructing the struct literal directly.
func NewBuildKitCacheMount(target string) BuildKitCacheMount {
	return BuildKitCacheMount{Target: target, Sharing: CacheTypeLocked}
}

package domain

import (
	"time"
)

// File represents a filesystem file discovered during repository scanning.
type File struct {
	RelativePath string
	Size         int64
	ModifiedTime time.Time
}

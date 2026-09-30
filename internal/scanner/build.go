package scanner

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"sync"
)

var executableBuild struct {
	once     sync.Once
	identity string
	err      error
}

// ExecutableBuildIdentity fingerprints the actual scanner executable once per
// process. Library consumers conservatively identify their containing binary;
// changing that binary requires re-extraction even if its schema is unchanged.
func ExecutableBuildIdentity() (string, error) {
	executableBuild.once.Do(func() {
		path, err := os.Executable()
		if err != nil {
			executableBuild.err = err
			return
		}
		// Linux exposes the running executable inode even if its disk pathname has
		// been replaced after startup. Other platforms use the executable path.
		if _, err := os.Stat("/proc/self/exe"); err == nil {
			path = "/proc/self/exe"
		}
		file, err := os.Open(path)
		if err != nil {
			executableBuild.err = err
			return
		}
		defer file.Close()
		hash := sha256.New()
		if _, err := io.Copy(hash, file); err != nil {
			executableBuild.err = err
			return
		}
		executableBuild.identity = fmt.Sprintf("sha256:%x", hash.Sum(nil))
	})
	return executableBuild.identity, executableBuild.err
}

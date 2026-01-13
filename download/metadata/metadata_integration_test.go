//go:build integration

package metadata

import (
	"os/exec"
)

// checkMutagenAvailable checks if the mutagen Python library is available.
// This is used by integration tests that require mutagen for metadata embedding.
func checkMutagenAvailable() error {
	cmd := exec.Command("python3", "-c", "import mutagen")
	return cmd.Run()
}

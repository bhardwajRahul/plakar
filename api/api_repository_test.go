package api

import (
	"os"
)

func init() {
	os.Setenv("TZ", "UTC")
}

// XXX: re-add TestRepositoryConfiguration once we move to non-mocked state object.

// XXX: re-add TestRepositorySnapshots once we move to non-mocked state object.

// XXX: re-add TestRepositorySnapshotsErrors once we move to non-mocked state object.

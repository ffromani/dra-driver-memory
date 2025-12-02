// this is taken from github.com/opencontainers/cgroups@e0c56cb31dcb3cb2b3d1554b20dd01ced32e2a2b
//
// local changes:
// - kept only TestOpenat2
// - always assume cgroups v2
// - use logr

package cgroups

import (
	"errors"
	"os"
	"testing"

	"github.com/go-logr/logr/testr"
)

func TestOpenat2(t *testing.T) {
	lh := testr.New(t)

	// Make sure we test openat2, not its fallback.
	openFallback = func(_ string, _ int, _ os.FileMode) (*os.File, error) {
		return nil, errors.New("fallback")
	}
	defer func() { openFallback = openAndCheck }()

	for _, tc := range []struct{ dir, file string }{
		{"/sys/fs/cgroup", "cgroup.controllers"},
		{"/sys/fs/cgroup", "/cgroup.controllers"},
		{"/sys/fs/cgroup/", "cgroup.controllers"},
		{"/sys/fs/cgroup/", "/cgroup.controllers"},
		{"/", "/sys/fs/cgroup/cgroup.controllers"},
		{"/", "sys/fs/cgroup/cgroup.controllers"},
		{"/sys/fs/cgroup/cgroup.controllers", ""},
	} {
		fd, err := OpenFile(lh, tc.dir, tc.file, os.O_RDONLY)
		if err != nil {
			t.Errorf("case %+v: %v", tc, err)
		}
		fd.Close() //nolint:errcheck
	}
}

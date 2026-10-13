//go:build unix

package diskfree

import "golang.org/x/sys/unix"

// freeBytes returns the space available to an unprivileged writer on the
// filesystem holding dir. Bavail (not Bfree) on purpose: the reserved blocks
// root can still use are not space this process can use.
func freeBytes(dir string) (uint64, error) {
	free, _, err := spaceBytes(dir)
	return free, err
}

func spaceBytes(dir string) (uint64, uint64, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(dir, &st); err != nil {
		return 0, 0, err
	}
	free := uint64(st.Bavail) * uint64(st.Bsize)
	total := uint64(st.Blocks) * uint64(st.Bsize)
	return free, total, nil
}


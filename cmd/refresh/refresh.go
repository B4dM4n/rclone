// +build linux darwin freebsd

package refresh

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/cmd/mount"
	"github.com/rclone/rclone/fs"
	"github.com/spf13/cobra"
)

var (
	recursive = false
)

func init() {
	cmd.Root.AddCommand(rmdirsCmd)
	rmdirsCmd.Flags().BoolVarP(&recursive, "recursive", "r", recursive, "Do recursive refresh")
}

var rmdirsCmd = &cobra.Command{
	Use:   "refresh path...",
	Short: `Refresh the dir cache for the given paths.`,
	Long: `This refreshes the directory cache for the given paths of a mounted remote.

If you supply the --recursive flag, it will do a recursive walk.

`,
	Run: func(command *cobra.Command, args []string) {
		exitCode := 0
		if len(args) == 0 {
			args = []string{"."}
		}
		for _, f := range args {
			abs, err := filepath.Abs(f)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: unable to get absolute path for %q: %s\n", f, err)
				exitCode = 1
				continue
			}
			err = ensureExists(abs)
			if err != nil {
				exitCode = 1
				continue
			}
			isFile, err := isFile(abs)
			if err != nil {
				exitCode = 1
				continue
			}
			if isFile {
				continue
			}
			if doIoctl(abs, recursive) != nil {
				exitCode = 1
			}
		}
		os.Exit(exitCode)
	},
}

func doIoctl(f string, recursive bool) error {
	r := mount.IocArgMagic
	if recursive {
		r |= 1
	}
	fd, err := os.Open(f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: unable to open %q: %s\n", f, err)
		return err
	}
	defer fs.CheckClose(fd, &err)
	result, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd.Fd(), uintptr(mount.IocRefresh), uintptr(r))
	if errno != 0 {
		fmt.Fprintf(os.Stderr, "Error: ioctl for %q: %s\n", f, errno)
		return errno
	} else if result != 0 {
		fmt.Fprintf(os.Stderr, "Error: refreshing %q failed. See rclone mount log for details", f)
		return fmt.Errorf("result")
	}
	return nil
}

func ensureExists(name string) error {
	for n := name; n != "."; n = filepath.Dir(n) {
		fi, err := os.Stat(n)
		switch {
		case os.IsNotExist(err):
			continue
		case err != nil:
			fmt.Fprintf(os.Stderr, "Error: stat for %q failed: %s\n", name, err)
			return err
		case !fi.IsDir():
			continue
		default:
			return doIoctl(n, false)
		}
	}
	fmt.Fprintf(os.Stderr, "Error: invalid path %q\n", name)
	return fmt.Errorf("invalid path")
}

func isFile(name string) (bool, error) {
	stat, err := os.Stat(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: stat for %q failed: %s\n", name, err)
		return false, err
	}
	return stat.Mode().IsRegular(), nil
}

// +build linux darwin freebsd

package refresh

import (
	"fmt"
	"os"
	"syscall"

	"github.com/ncw/rclone/cmd"
	"github.com/ncw/rclone/cmd/mount"
	"github.com/ncw/rclone/fs"
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
			if doIoctl(f) != nil {
				exitCode = 1
			}
		}
		os.Exit(exitCode)
	},
}

func doIoctl(f string) error {
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

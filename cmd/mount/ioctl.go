package mount

import (
	"bazil.org/fuse"
	fusefs "bazil.org/fuse/fs"
	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/log"
	"golang.org/x/net/context"
)

// Check interface satisfied
var _ fusefs.NodeIoctler = (*Dir)(nil)

// Ioctl handles ioctl calls
func (f *File) Ioctl(ctx context.Context, req *fuse.IoctlRequest, resp *fuse.IoctlResponse) (err error) {
	defer log.Trace(f, "req=%v", req)("resp=%v, err=%v", &resp, &err)

	switch req.Cmd {
	case IocRefresh:
		if !IocCheckMagic(req.Arg) {
			return fuse.ENOTTY
		}

		if err := f.Dir().ReadDir(); err != nil {
			fs.Logf(f, "ReadDir: %s", err)
			resp.Result = 1
		}
		return nil
	default:
		return fuse.ENOTTY
	}
}

// Ioctl handles ioctl calls
func (d *Dir) Ioctl(ctx context.Context, req *fuse.IoctlRequest, resp *fuse.IoctlResponse) (err error) {
	defer log.Trace(d, "req=%v", req)("resp=%v, err=%v", &resp, &err)

	switch req.Cmd {
	case IocRefresh:
		if !IocCheckMagic(req.Arg) {
			return fuse.ENOTTY
		}

		if req.Arg&1 != 0 {
			if err := d.ReadDirTree(); err != nil {
				fs.Logf(d, "ReadDirTree: %s", err)
				resp.Result = 1
			}
		} else {
			if err := d.ReadDir(); err != nil {
				fs.Logf(d, "ReadDir: %s", err)
				resp.Result = 1
			}
		}

		return nil
	default:
		return fuse.ENOTTY
	}
}

type iocDir byte

const (
	iocDirNone iocDir = iota
	iocDirWrite
	iocDirRead
)

func makeIoc(dir iocDir, typ, nr byte, size uint16) uint32 {
	return (uint32(dir) << iocDirshift) |
		(uint32(typ) << iocTypeshift) |
		(uint32(nr) << iocNrshift) |
		(uint32(size) << iocSizeshift)
}

func makeIocN(typ, nr byte) uint32 { // nolint
	return makeIoc(iocDirNone, typ, nr, 0)
}
func makeIocW(typ, nr byte, size uint16) uint32 { // nolint
	return makeIoc(iocDirWrite, typ, nr, size)
}
func makeIocR(typ, nr byte, size uint16) uint32 { // nolint
	return makeIoc(iocDirRead, typ, nr, size)
}
func decodeIoc(ioc uint32) (dir iocDir, typ, nr byte, size uint16) { // nolint
	return iocDir((ioc >> iocDirshift) & iocDirmask),
		byte((ioc >> iocTypeshift) & iocTypemask),
		byte((ioc >> iocNrshift) & iocNrmask),
		uint16((ioc >> iocSizeshift) & iocSizemask)
}

// IocCheckMagic returns true when the bits 16-31 contain the IocArgMagic value.
func IocCheckMagic(arg uint64) bool {
	return arg&IocArgMagicMask == IocArgMagic
}

const (
	iocDirbits  = 2
	iocSizebits = 14
	iocTypebits = 8
	iocNrbits   = 8

	iocNrmask   = (1 << iocNrbits) - 1
	iocTypemask = (1 << iocTypebits) - 1
	iocSizemask = (1 << iocSizebits) - 1
	iocDirmask  = (1 << iocDirbits) - 1

	iocNrshift   = 0
	iocTypeshift = iocNrshift + iocNrbits
	iocSizeshift = iocTypeshift + iocTypebits
	iocDirshift  = iocSizeshift + iocSizebits
)

// IocRefresh is the vfs refresh ioctl code
var IocRefresh = makeIocN('r', 0x20)

// Magic number to differentiate foreign ioctl requests
const (
	IocArgMagic     = 'c'<<24 | 'l'<<16
	IocArgMagicMask = 0xFFFF << 16
)

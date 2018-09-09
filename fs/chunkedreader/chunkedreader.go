package chunkedreader

import (
	"errors"
	"io"
	"math"
	"sync"

	"github.com/ncw/rclone/fs"
)

// io related errors returned by ChunkedReader
var (
	ErrorFileClosed  = errors.New("file already closed")
	ErrorInvalidSeek = errors.New("invalid seek position")
)

// ChunkedReader is a reader for a Object with the possibility
// of reading the source in chunks of given size
type ChunkedReader struct {
	mu          sync.Mutex        // protects following fields
	o           fs.Object         // source to read from
	rc          io.ReadCloser     // reader for the current open chunk
	offset      int64             // offset the next Read will start. -1 forces a reopen of o
	chunkOffset int64             // beginning of the current or next chunk
	chunkSize   int64             // length of the current or next chunk. -1 will open o from chunkOffset to the end
	sizeIter    ChunkSizeIterator // function to calculate the next chunk size
	closed      bool              // has Close been called?
}

// ChunkSizeIterator is used to calculate the chunk size values.
type ChunkSizeIterator interface {
	// NextChunkSize returns the next chunk size to use.
	// A return value <= 0 will disable chunk handling.
	NextChunkSize() int64
	// Reset will be called after RangeSeek with the last used chunk size.
	Reset(int64)
}

type minMaxIterator struct {
	cur, min, max int64
}

func (mmi *minMaxIterator) NextChunkSize() int64 {
	if mmi.cur < mmi.min || mmi.min == -1 {
		mmi.cur = mmi.min
		return mmi.min
	}
	if mmi.cur > 0 {
		mmi.cur *= 2
	}
	if mmi.cur > mmi.max {
		return mmi.max
	}
	return mmi.cur
}
func (mmi *minMaxIterator) Reset(int64) {
	mmi.cur = 0
}

// IteratorFromMinMax returns a combination of Min, Max and Mulitply by 2 SizeFunc's.
// A min of <= 0 will always return -1 and disable chunked reading.
// If max is greater than min, the last value will be doubled each time.
func IteratorFromMinMax(min, max int64) ChunkSizeIterator {
	if min <= 0 {
		return &minMaxIterator{
			min: -1,
		}
	}
	if max != -1 {
		if max < min {
			max = min
		}
	} else {
		max = math.MaxInt64
	}
	return &minMaxIterator{
		min: min,
		max: max,
	}
}
func fixNeg(size int64) int64 {
	if size <= 0 {
		return -1
	}
	return size
}

// New returns a ChunkedReader for the Object.
//
// A initialChunkSize of <= 0 will disable chunked reading.
// If maxChunkSize is greater than initialChunkSize, the chunk size will be
// doubled after each chunk read with a maximun of maxChunkSize.
// A Seek or RangeSeek will reset the chunk size to it's initial value
func New(o fs.Object, initialChunkSize, maxChunkSize int64) *ChunkedReader {
	return NewWithChunkSizeIterator(o, IteratorFromMinMax(initialChunkSize, maxChunkSize))
}

// NewWithChunkSizeIterator returns a ChunkedReader for the Object and
// the given chunk size calculation.
// When the sizeIter returns a value <= 0, chunked reading is disabled.
func NewWithChunkSizeIterator(o fs.Object, sizeIter ChunkSizeIterator) *ChunkedReader {
	return &ChunkedReader{
		o:         o,
		offset:    -1,
		chunkSize: fixNeg(sizeIter.NextChunkSize()),
		sizeIter:  sizeIter,
	}
}

// Read from the file - for details see io.Reader
func (cr *ChunkedReader) Read(p []byte) (n int, err error) {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	if cr.closed {
		return 0, ErrorFileClosed
	}

	for reqSize := int64(len(p)); reqSize > 0; reqSize = int64(len(p)) {
		// the current chunk boundary. valid only when chunkSize > 0
		chunkEnd := cr.chunkOffset + cr.chunkSize

		fs.Debugf(cr.o, "ChunkedReader.Read at %d length %d chunkOffset %d chunkSize %d", cr.offset, reqSize, cr.chunkOffset, cr.chunkSize)

		switch {
		case cr.chunkSize > 0 && cr.offset == chunkEnd: // last chunk read completely
			cr.chunkOffset = cr.offset
			cr.chunkSize = fixNeg(cr.sizeIter.NextChunkSize())
			// recalculate the chunk boundary. valid only when chunkSize > 0
			chunkEnd = cr.chunkOffset + cr.chunkSize
			fallthrough
		case cr.offset == -1: // first Read or Read after RangeSeek
			err = cr.openRange()
			if err != nil {
				return
			}
		}

		var buf []byte
		chunkRest := chunkEnd - cr.offset
		// limit read to chunk boundaries if chunkSize > 0
		if reqSize > chunkRest && cr.chunkSize > 0 {
			buf, p = p[0:chunkRest], p[chunkRest:]
		} else {
			buf, p = p, nil
		}
		var rn int
		rn, err = io.ReadFull(cr.rc, buf)
		n += rn
		cr.offset += int64(rn)
		if err != nil {
			if err == io.ErrUnexpectedEOF {
				err = io.EOF
			}
			return
		}
	}
	return n, nil
}

// Close the file - for details see io.Closer
//
// All methods on ChunkedReader will return ErrorFileClosed afterwards
func (cr *ChunkedReader) Close() error {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	if cr.closed {
		return ErrorFileClosed
	}
	cr.closed = true

	return cr.resetReader(nil, 0)
}

// Seek the file - for details see io.Seeker
func (cr *ChunkedReader) Seek(offset int64, whence int) (int64, error) {
	return cr.RangeSeek(offset, whence, -1)
}

// RangeSeek the file - for details see RangeSeeker
//
// The specified length will only apply to the next chunk opened.
// RangeSeek will not reopen the source until Read is called.
func (cr *ChunkedReader) RangeSeek(offset int64, whence int, length int64) (int64, error) {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	fs.Debugf(cr.o, "ChunkedReader.RangeSeek from %d to %d length %d", cr.offset, offset, length)

	if cr.closed {
		return 0, ErrorFileClosed
	}

	size := cr.o.Size()
	switch whence {
	case io.SeekStart:
		cr.offset = 0
	case io.SeekEnd:
		cr.offset = size
	}
	// set the new chunk start
	cr.chunkOffset = cr.offset + offset
	// force reopen on next Read
	cr.offset = -1
	cr.sizeIter.Reset(length)
	if length > 0 {
		cr.chunkSize = length
	} else {
		cr.chunkSize = fixNeg(cr.sizeIter.NextChunkSize())
	}
	if cr.chunkOffset < 0 || cr.chunkOffset >= size {
		cr.chunkOffset = 0
		return 0, ErrorInvalidSeek
	}
	return cr.chunkOffset, nil
}

// Open forces the connection to be opened
func (cr *ChunkedReader) Open() (*ChunkedReader, error) {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	if cr.rc != nil && cr.offset != -1 {
		return cr, nil
	}
	return cr, cr.openRange()
}

// openRange will open the source Object with the current chunk range
//
// If the current open reader implenets RangeSeeker, it is tried first.
// When RangeSeek failes, o.Open with a RangeOption is used.
//
// A length <= 0 will request till the end of the file
func (cr *ChunkedReader) openRange() error {
	offset, length := cr.chunkOffset, cr.chunkSize
	fs.Debugf(cr.o, "ChunkedReader.openRange at %d length %d", offset, length)

	if cr.closed {
		return ErrorFileClosed
	}

	if rs, ok := cr.rc.(fs.RangeSeeker); ok {
		n, err := rs.RangeSeek(offset, io.SeekStart, length)
		if err == nil && n == offset {
			cr.offset = offset
			return nil
		}
		if err != nil {
			fs.Debugf(cr.o, "ChunkedReader.openRange seek failed (%s). Trying Open", err)
		} else {
			fs.Debugf(cr.o, "ChunkedReader.openRange seeked to wrong offset. Wanted %d, got %d. Trying Open", offset, n)
		}
	}

	var rc io.ReadCloser
	var err error
	if length <= 0 {
		if offset == 0 {
			rc, err = cr.o.Open()
		} else {
			rc, err = cr.o.Open(&fs.RangeOption{Start: offset, End: -1})
		}
	} else {
		rc, err = cr.o.Open(&fs.RangeOption{Start: offset, End: offset + length - 1})
	}
	if err != nil {
		return err
	}
	return cr.resetReader(rc, offset)
}

// resetReader switches the current reader to the given reader.
// The old reader will be Close'd before setting the new reader.
func (cr *ChunkedReader) resetReader(rc io.ReadCloser, offset int64) error {
	if cr.rc != nil {
		if err := cr.rc.Close(); err != nil {
			return err
		}
	}
	cr.rc = rc
	cr.offset = offset
	return nil
}

var (
	_ io.ReadCloser  = (*ChunkedReader)(nil)
	_ io.Seeker      = (*ChunkedReader)(nil)
	_ fs.RangeSeeker = (*ChunkedReader)(nil)
)

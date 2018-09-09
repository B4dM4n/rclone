package chunkedreader

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/ncw/rclone/fs"
	"github.com/pkg/errors"
)

// MultiplierList represents a list of size values, where two values
// can be interpolated by using a multiplier
type MultiplierList struct {
	anchors     []int64
	multipliers []int64
}

type multiplierListIter struct {
	MultiplierList
	sliceIdx int
	lastVal  int64
}

// ParseMultiplierList parses a comma separeated list of fs.SizeSuffix values.
// Between two values or at the end, a multiplier can be insterted by prefixing
// a int value with a "x".
func ParseMultiplierList(s string) (*MultiplierList, error) {
	return ParseMultiplierListParts(strings.Split(s, ","))
}

// ParseMultiplierListParts parses a list of fs.SizeSuffix values.
// Between two values or at the end, a multiplier can be insterted by prefixing
// a int value with a "x".
func ParseMultiplierListParts(parts []string) (*MultiplierList, error) {
	var (
		anchors     []int64
		multipliers []int64
	)
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			return nil, errors.New("empty segment")
		}
		mul := false
		if p[0] == 'x' {
			p = p[1:]
			mul = true
		}
		if mul {
			if len(multipliers) == 0 {
				return nil, errors.New("multiplier at first position")
			}
			if multipliers[len(multipliers)-1] != 0 {
				return nil, errors.New("multiple multipliers in a row")
			}
			i, err := strconv.ParseInt(p, 10, 64)
			if err != nil {
				return nil, errors.Wrapf(err, "invalid multiplier %s", p)
			}
			if i < 2 {
				return nil, errors.Wrapf(err, "invalid multiplier %s", p)
			}
			multipliers[len(multipliers)-1] = i
		} else {
			var ss fs.SizeSuffix
			err := ss.Set(p)
			if err != nil {
				return nil, errors.Wrapf(err, "invalid size %s", p)
			}
			if ss <= 0 {
				return nil, errors.Errorf("invalid size %s", p)
			}
			anchors = append(anchors, int64(ss))
			multipliers = append(multipliers, 0)
		}
	}
	return &MultiplierList{
		anchors:     anchors,
		multipliers: multipliers,
	}, nil
}

// Iter returns a ChunkSizeIterator for this list.
func (ml *MultiplierList) Iter() ChunkSizeIterator {
	return &multiplierListIter{
		MultiplierList: *ml,
	}
}

func (ml *multiplierListIter) NextChunkSize() int64 {
	l := len(ml.anchors)
	if l == 0 {
		return -1
	}
	if ml.sliceIdx >= l {
		m := ml.multipliers[l-1]
		if m > 1 {
			ml.lastVal *= m
		}
		return int64(ml.lastVal)
	}
	if ml.lastVal == 0 {
		ml.lastVal = ml.anchors[0]
		return int64(ml.lastVal)
	}

	m := ml.multipliers[ml.sliceIdx]
	if m <= 1 {
		ml.sliceIdx++
		if ml.sliceIdx < l {
			ml.lastVal = ml.anchors[ml.sliceIdx]
		}
		return int64(ml.lastVal)
	}

	ml.lastVal *= m
	if ml.sliceIdx+1 < l {
		n := ml.anchors[ml.sliceIdx+1]
		if ml.lastVal >= n {
			ml.lastVal = n
			ml.sliceIdx++
		}
	}
	return int64(ml.lastVal)
}
func (ml *multiplierListIter) Reset(int64) {
	ml.sliceIdx, ml.lastVal = 0, 0
}

// Empty returns true when the list has no entries
func (ml *MultiplierList) Empty() bool {
	return len(ml.anchors) == 0
}

// Type of the value
func (MultiplierList) Type() string {
	return "string"
}

// Scan implements the fmt.Scanner interface
func (ml *MultiplierList) Scan(s fmt.ScanState, ch rune) error {
	token, err := s.Token(true, func(rune) bool { return true })
	if err != nil {
		return err
	}
	return ml.Set(strings.TrimSpace(string(token)))
}

func (ml MultiplierList) String() string {
	var buf bytes.Buffer
	for i, a := range ml.anchors {
		if i != 0 {
			buf.WriteByte(',')
		}
		buf.WriteString(fs.SizeSuffix(a).String())
		m := ml.multipliers[i]
		if m > 1 {
			_, _ = fmt.Fprintf(&buf, ",x%d", m)
		}
	}
	return buf.String()
}

// Set the List entries
func (ml *MultiplierList) Set(s string) error {
	n, err := ParseMultiplierList(s)
	if err != nil {
		return err
	}
	*ml = *n
	return nil
}

//go:build windows
// +build windows

package windrive

import (
	"fmt"

	"golang.org/x/sys/windows"
)

type Partition struct {
	Name       string
	Path       string
	Removable  bool
	FileSystem FileSystem
}

func (p Partition) String() string {
	return fmt.Sprintf("%s (%s)", p.Name, p.Path)
}

func getPartitionsPath() ([][]uint16, error) {
	buf := make([]uint16, 128)

	for {
		l := uint32(len(buf))

		n, err := windows.GetLogicalDriveStrings(l, &buf[0])
		if err != nil {
			return nil, err
		}

		if n > l {
			buf = make([]uint16, n)
			continue
		}

		res := make([][]uint16, 0)
		l = 0

		for {
			if l >= n {
				break
			}

			if buf[l] != 0 {
				l++
				continue
			}

			l++
			if l > 1 {
				res = append(res, buf[:l])
			}

			buf = buf[l:]
			n -= l
			l = 0
		}

		return res, nil
	}
}

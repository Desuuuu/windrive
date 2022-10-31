//go:build windows
// +build windows

package windrive

import (
	"golang.org/x/sys/windows"
)

// Paths returns the path of every available drive.
func Paths() ([]string, error) {
	drivePaths, err := getDrivePaths()
	if err != nil {
		return nil, err
	}

	res := make([]string, 0, len(drivePaths))
	for _, drivePath := range drivePaths {
		res = append(res, windows.UTF16ToString(drivePath))
	}

	return res, nil
}

func getDrivePaths() ([][]uint16, error) {
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

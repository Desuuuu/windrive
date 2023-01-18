//go:build windows
// +build windows

package windrive

import (
	"golang.org/x/sys/windows"
)

type FileSystem struct {
	Kind               string
	Flags              uint32
	MaxComponentLength uint32
}

func (fs FileSystem) IsReadOnly() bool {
	return (fs.Flags & windows.FILE_READ_ONLY_VOLUME) != 0
}

func getPartitionInformation(path *uint16) (string, FileSystem, error) {
	var flags, maximumComponentLength uint32

	name := make([]uint16, windows.MAX_PATH+1)
	kind := make([]uint16, windows.MAX_PATH+1)

	err := windows.GetVolumeInformation(path, &name[0], uint32(len(name)), nil, &maximumComponentLength, &flags, &kind[0], uint32(len(kind)))
	if err != nil {
		return "", FileSystem{}, err
	}

	return windows.UTF16ToString(name), FileSystem{
		Kind:               windows.UTF16ToString(kind),
		Flags:              flags,
		MaxComponentLength: maximumComponentLength,
	}, nil
}

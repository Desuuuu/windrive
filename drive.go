//go:build windows
// +build windows

package windrive

import (
	"golang.org/x/sys/windows"
)

type DriveType int

const (
	DriveTypeInvalid DriveType = iota
	DriveTypeRemovable
	DriveTypeFixed
	DriveTypeRemote
	DriveTypeCDRom
	DriveTypeRAMDisk
)

type Drive struct {
	Path       string
	Type       DriveType
	VolumeName string
	FileSystem FileSystem
}

type FileSystem struct {
	Name               string
	Flags              uint32
	MaxComponentLength uint32
}

func (fs FileSystem) IsReadOnly() bool {
	return (fs.Flags & windows.FILE_READ_ONLY_VOLUME) != 0
}

// List returns the list of available drives, optionally filtered by type.
func List(driveTypes ...DriveType) ([]*Drive, error) {
	drivePaths, err := getDrivePaths()
	if err != nil {
		return nil, err
	}

	var res []*Drive

	filter := make(map[DriveType]struct{}, len(driveTypes))
	for _, driveType := range driveTypes {
		filter[driveType] = struct{}{}
	}

	for _, drivePath := range drivePaths {
		driveType := getDriveType(windows.GetDriveType(&drivePath[0]))
		if driveType == DriveTypeInvalid {
			continue
		}

		if len(filter) > 0 {
			if _, ok := filter[driveType]; !ok {
				continue
			}
		}

		volumeName, fileSystem, err := getVolumeInformation(&drivePath[0])
		if err != nil {
			continue
		}

		res = append(res, &Drive{
			Path:       windows.UTF16ToString(drivePath),
			Type:       driveType,
			VolumeName: volumeName,
			FileSystem: fileSystem,
		})
	}

	return res, nil
}

func getDriveType(driveType uint32) DriveType {
	switch driveType {
	case windows.DRIVE_REMOVABLE:
		return DriveTypeRemovable
	case windows.DRIVE_FIXED:
		return DriveTypeFixed
	case windows.DRIVE_REMOTE:
		return DriveTypeRemote
	case windows.DRIVE_CDROM:
		return DriveTypeCDRom
	case windows.DRIVE_RAMDISK:
		return DriveTypeRAMDisk
	}

	return DriveTypeInvalid
}

func getVolumeInformation(rootPathName *uint16) (string, FileSystem, error) {
	var maximumComponentLength uint32
	var fileSystemFlags uint32

	volumeName := make([]uint16, windows.MAX_PATH+1)
	fileSystemName := make([]uint16, windows.MAX_PATH+1)

	err := windows.GetVolumeInformation(rootPathName, &volumeName[0], uint32(len(volumeName)), nil, &maximumComponentLength, &fileSystemFlags, &fileSystemName[0], uint32(len(fileSystemName)))
	if err != nil {
		return "", FileSystem{}, err
	}

	return windows.UTF16ToString(volumeName), FileSystem{
		Name:               windows.UTF16ToString(fileSystemName),
		Flags:              fileSystemFlags,
		MaxComponentLength: maximumComponentLength,
	}, nil
}

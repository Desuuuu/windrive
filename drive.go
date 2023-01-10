//go:build windows
// +build windows

package windrive

import (
	"errors"
	"fmt"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

type Drive struct {
	Name       string
	Path       string
	Partitions []*Partition
}

func (d Drive) String() string {
	return d.Name
}

// List returns the physical drives currently connected. Drives without any
// partition are not included.
func List() ([]*Drive, error) {
	partitionsPath, err := getPartitionsPath()
	if err != nil {
		return nil, err
	}

	drives := make(map[string]*Drive)

	for _, partitionPath := range partitionsPath {
		driveType := windows.GetDriveType(&partitionPath[0])
		if driveType != windows.DRIVE_REMOVABLE && driveType != windows.DRIVE_FIXED {
			continue
		}

		name, fileSystem, err := getPartitionInformation(&partitionPath[0])
		if err != nil {
			continue
		}

		partition := &Partition{
			Name:       name,
			Path:       strings.TrimRight(windows.UTF16ToString(partitionPath), `\`),
			Removable:  driveType == windows.DRIVE_REMOVABLE,
			FileSystem: fileSystem,
		}

		drivePath, err := getDrivePath(partition.Path)
		if err != nil {
			continue
		}

		if drive, ok := drives[drivePath]; ok {
			drive.Partitions = append(drive.Partitions, partition)
		} else {
			driveName, err := getDriveName(partition.Path)
			if err != nil {
				continue
			}

			drives[drivePath] = &Drive{
				Name: driveName,
				Path: drivePath,
				Partitions: []*Partition{
					partition,
				},
			}
		}
	}

	res := make([]*Drive, 0, len(drives))
	for _, drive := range drives {
		res = append(res, drive)
	}

	return res, nil
}

const (
	ioctl_storage_get_device_number = 0x002d1080
	ioctl_storage_query_property    = 0x002d1400
	storageDeviceProperty           = 0
	propertyStandardQuery           = 0
	file_device_disk                = 0x00000007
)

type storagePropertyQuery struct {
	PropertyId uint
	QueryType  uint
}

type storageDescriptorHeader struct {
	Version uint32
	Size    uint32
}

type storageDeviceNumber struct {
	DeviceType      uint32
	DeviceNumber    uint32
	PartitionNumber uint32
}

func getDriveName(path string) (string, error) {
	path = fmt.Sprintf(`\\.\%s`, path)

	fd, err := windows.CreateFile(
		windows.StringToUTF16Ptr(path),
		0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = windows.CloseHandle(fd)
	}()

	spq := storagePropertyQuery{
		PropertyId: storageDeviceProperty,
		QueryType:  propertyStandardQuery,
	}

	var sdh storageDescriptorHeader
	sdhSize := uint32(unsafe.Sizeof(sdh))

	var bytesReturned uint32

	err = windows.DeviceIoControl(
		fd,
		ioctl_storage_query_property,
		(*byte)(unsafe.Pointer(&spq)),
		uint32(unsafe.Sizeof(spq)),
		(*byte)(unsafe.Pointer(&sdh)),
		sdhSize,
		&bytesReturned,
		nil,
	)
	if err != nil {
		return "", err
	}

	if bytesReturned < sdhSize {
		return "", errors.New("invalid response")
	}

	if sdh.Size < 16 {
		return "", nil
	}

	buf := make([]byte, sdh.Size)

	err = windows.DeviceIoControl(
		fd,
		ioctl_storage_query_property,
		(*byte)(unsafe.Pointer(&spq)),
		uint32(unsafe.Sizeof(spq)),
		&buf[0],
		sdh.Size,
		&bytesReturned,
		nil,
	)
	if err != nil {
		return "", err
	}

	if bytesReturned < sdh.Size {
		return "", errors.New("invalid response")
	}

	var vendorId, productId string

	if offset := *(*uint32)(unsafe.Pointer(&buf[12])); offset > 0 {
		vendorId = strings.TrimSpace(windows.BytePtrToString(&buf[offset]))
	}

	if sdh.Size >= 20 {
		if offset := *(*uint32)(unsafe.Pointer(&buf[16])); offset > 0 {
			productId = strings.TrimSpace(windows.BytePtrToString(&buf[offset]))
		}
	}

	if vendorId != "" {
		if productId != "" {
			return fmt.Sprintf("%s %s", vendorId, productId), nil
		}

		return vendorId, nil
	}

	return productId, nil
}

func getDrivePath(path string) (string, error) {
	path = fmt.Sprintf(`\\.\%s`, path)

	fd, err := windows.CreateFile(
		windows.StringToUTF16Ptr(path),
		0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = windows.CloseHandle(fd)
	}()

	var sdn storageDeviceNumber
	sdnSize := uint32(unsafe.Sizeof(sdn))

	var bytesReturned uint32

	err = windows.DeviceIoControl(
		fd,
		ioctl_storage_get_device_number,
		nil,
		0,
		(*byte)(unsafe.Pointer(&sdn)),
		sdnSize,
		&bytesReturned,
		nil,
	)
	if err != nil {
		return "", err
	}

	if bytesReturned < sdnSize {
		return "", errors.New("invalid response")
	}

	if sdn.DeviceType != file_device_disk {
		return "", errors.New("invalid device")
	}

	return fmt.Sprintf(`\\.\PhysicalDrive%d`, sdn.DeviceNumber), nil
}

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

func (d Drive) IsRemovable() bool {
	for _, partition := range d.Partitions {
		if !partition.Removable {
			return false
		}
	}

	return true
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

		fd, err := openDrive(partition.Path)
		if err != nil {
			continue
		}

		drivePath, err := getDrivePath(fd)
		if err != nil {
			_ = windows.CloseHandle(fd)
			continue
		}

		if drive, ok := drives[drivePath]; ok {
			drive.Partitions = append(drive.Partitions, partition)
		} else {
			driveName, err := getDriveName(fd)
			if err != nil {
				_ = windows.CloseHandle(fd)
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

		_ = windows.CloseHandle(fd)
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

func openDrive(path string) (windows.Handle, error) {
	path = fmt.Sprintf(`\\.\%s`, path)

	return windows.CreateFile(
		windows.StringToUTF16Ptr(path),
		0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
}

type storageDeviceNumber struct {
	DeviceType      uint32
	DeviceNumber    uint32
	PartitionNumber uint32
}

func getDrivePath(fd windows.Handle) (string, error) {
	var sdn storageDeviceNumber
	sdnSize := uint32(unsafe.Sizeof(sdn))

	var bytesReturned uint32

	err := windows.DeviceIoControl(
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

type storagePropertyQuery struct {
	PropertyId           uint32
	QueryType            uint32
	AdditionalParameters uint8
}

type storageDescriptorHeader struct {
	Version uint32
	Size    uint32
}

func getDriveName(fd windows.Handle) (string, error) {
	spq := storagePropertyQuery{
		PropertyId: storageDeviceProperty,
		QueryType:  propertyStandardQuery,
	}

	var sdh storageDescriptorHeader
	sdhSize := uint32(unsafe.Sizeof(sdh))

	var bytesReturned uint32

	err := windows.DeviceIoControl(
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
		return "", nil
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
		return "", nil
	}

	var vendorId, productId string

	if offset := *(*uint32)(unsafe.Pointer(&buf[12])); offset > 0 && offset < sdh.Size {
		vendorId = strings.TrimSpace(windows.BytePtrToString(&buf[offset]))
	}

	if sdh.Size >= 20 {
		if offset := *(*uint32)(unsafe.Pointer(&buf[16])); offset > 0 && offset < sdh.Size {
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

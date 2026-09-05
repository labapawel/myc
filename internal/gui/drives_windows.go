//go:build windows

package gui

import (
	"golang.org/x/sys/windows"
)

func getDiskSpace(path string) (int64, int64) {
	var freeBytesAvailable, totalNumberOfBytes, totalNumberOfFreeBytes uint64
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, 0
	}
	err = windows.GetDiskFreeSpaceEx(pathPtr, &freeBytesAvailable, &totalNumberOfBytes, &totalNumberOfFreeBytes)
	if err != nil {
		return 0, 0
	}
	return int64(totalNumberOfBytes), int64(freeBytesAvailable)
}

func GetDrives() []DriveInfo {
	var drives []DriveInfo
	mask, err := windows.GetLogicalDrives()
	if err != nil {
		for r := 'A'; r <= 'Z'; r++ {
			drivePath := string(r) + ":\\"
			total, free := getDiskSpace(drivePath)
			if total > 0 {
				drives = append(drives, DriveInfo{
					Letter:    string(r),
					Path:      drivePath,
					Name:      string(r) + ":",
					TotalSize: total,
					FreeSize:  free,
				})
			}
		}
		return drives
	}

	for i := 0; i < 26; i++ {
		if (mask & (1 << i)) != 0 {
			letter := string(rune('A' + i))
			drivePath := letter + ":\\"
			total, free := getDiskSpace(drivePath)
			drives = append(drives, DriveInfo{
				Letter:    letter,
				Path:      drivePath,
				Name:      letter + ":",
				TotalSize: total,
				FreeSize:  free,
			})
		}
	}

	return drives
}

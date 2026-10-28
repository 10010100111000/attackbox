package runtime

import (
	"bufio"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

var mountPathUnescaper = strings.NewReplacer(
	`\040`, " ",
	`\011`, "\t",
	`\012`, "\n",
	`\134`, `\`,
)

func pathHasMountPrefix(targetPath, mountPoint string) bool {
	if mountPoint == "/" {
		return strings.HasPrefix(targetPath, "/")
	}
	return targetPath == mountPoint || strings.HasPrefix(targetPath, mountPoint+"/")
}

func findBackingDevice(r io.Reader, targetPath string) (bestMountPoint, bestDevSource, fallbackDevNum string) {
	cleanTarget := path.Clean(filepath.ToSlash(targetPath))
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) < 7 {
			continue
		}
		devNum := parts[2]
		mountPoint := path.Clean(mountPathUnescaper.Replace(parts[4]))

		if pathHasMountPrefix(cleanTarget, mountPoint) && len(mountPoint) >= len(bestMountPoint) {
			bestMountPoint = mountPoint
			fallbackDevNum = devNum
			bestDevSource = ""

			// Find optional fields separator "-"
			for i := 6; i < len(parts); i++ {
				if parts[i] == "-" && i+2 < len(parts) {
					bestDevSource = mountPathUnescaper.Replace(parts[i+2])
					break
				}
			}
		}
	}
	return bestMountPoint, bestDevSource, fallbackDevNum
}

// DetectDeviceNumber detects the underlying physical block device major:minor (whole disk) for a given directory.
// For filesystems like Btrfs where mountinfo reports anonymous virtual device (0:X), it extracts the real
// backing device path (e.g. /dev/vda1) and traverses sysfs to find the whole disk device number (e.g. 254:0).
func DetectDeviceNumber(dirPath string) string {
	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return ""
	}
	defer f.Close()

	absPath, err := filepath.Abs(dirPath)
	if err != nil {
		absPath = dirPath
	}

	_, bestDevSource, fallbackDevNum := findBackingDevice(f, absPath)

	if strings.HasPrefix(bestDevSource, "/dev/") {
		devName := filepath.Base(bestDevSource)
		sysBlockPath := filepath.Join("/sys/class/block", devName)
		realPath, err := filepath.EvalSymlinks(sysBlockPath)
		if err == nil {
			// Check if it's a partition (partitions have a "partition" sysfs attribute file)
			partitionFile := filepath.Join(realPath, "partition")
			if _, err := os.Stat(partitionFile); err == nil {
				// It's a partition; the parent directory is the whole disk device
				parentDevFile := filepath.Join(filepath.Dir(realPath), "dev")
				if data, err := os.ReadFile(parentDevFile); err == nil {
					val := strings.TrimSpace(string(data))
					if val != "" {
						return val
					}
				}
			}

			// Whole disk direct dev
			devFile := filepath.Join(realPath, "dev")
			if data, err := os.ReadFile(devFile); err == nil {
				val := strings.TrimSpace(string(data))
				if val != "" {
					return val
				}
			}
		}
	}

	// Fallback to non-zero mountinfo major device if sysfs not available
	if fallbackDevNum != "" && !strings.HasPrefix(fallbackDevNum, "0:") {
		return fallbackDevNum
	}
	return ""
}

package system

import (
	"fmt"
	"strings"
)

func TailFile(path string, lines int) []string {
	result := Execute("tail", "-n", fmt.Sprintf("%d", lines), path)
	if !result.Success {
		return []string{"Unable to read log file"}
	}
	output := strings.Split(result.Stdout, "\n")
	return output
}

func DiskUsage(path string) (total, used, free uint64, err error) {
	result := ExecuteBash("df --block-size=1 " + path + " | tail -1 | awk '{print $2,$3,$4}'")
	if !result.Success {
		return 0, 0, 0, fmt.Errorf("failed to get disk usage")
	}

	parts := strings.Fields(result.Stdout)
	if len(parts) == 3 {
		total = parseSize(parts[0])
		used = parseSize(parts[1])
		free = parseSize(parts[2])
	}
	return
}

func parseSize(s string) uint64 {
	var val uint64
	fmt.Sscanf(s, "%d", &val)
	return val
}

func GetOSInfo() map[string]string {
	info := make(map[string]string)

	result := ExecuteBash("cat /etc/os-release 2>/dev/null | head -5 | tr '\\n' ' '")
	if result.Success {
		info["os"] = result.Stdout
	}

	result = Execute("uname", "-r")
	if result.Success {
		info["kernel"] = result.Stdout
	}

	result = Execute("uptime", "-p")
	if result.Success {
		info["uptime"] = result.Stdout
	}

	hostname := Execute("hostname")
	if hostname.Success {
		info["hostname"] = hostname.Stdout
	}

	return info
}

func GetMemoryInfo() map[string]uint64 {
	info := make(map[string]uint64)
	result := ExecuteBash("free -b | grep Mem | awk '{print $2,$3,$4,$7}'")
	if result.Success {
		parts := strings.Fields(result.Stdout)
		if len(parts) == 4 {
			info["total"] = parseSize(parts[0])
			info["used"] = parseSize(parts[1])
			info["free"] = parseSize(parts[2])
			info["available"] = parseSize(parts[3])
		}
	}
	return info
}

func GetCPUUsage() float64 {
	result := ExecuteBash("top -bn1 | grep 'Cpu(s)' | awk '{print $2}' | cut -d'%' -f1")
	if result.Success {
		var usage float64
		fmt.Sscanf(result.Stdout, "%f", &usage)
		return usage
	}
	return 0
}

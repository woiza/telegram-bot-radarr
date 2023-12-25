package utils

import (
	"fmt"
	"strings"

	"golift.io/starr/radarr"
)

func Escape(text string) string {
	var specialChars = "()[]{}_-*~`><&#+=|!.\\"
	var escaped strings.Builder
	for _, ch := range text {
		if strings.ContainsRune(specialChars, ch) {
			escaped.WriteRune('\\')
		}
		escaped.WriteRune(ch)
	}
	return escaped.String()
}

func ByteCountSI(b int64) string {
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "kMGTPE"[exp])
}

func PrepareRootFolders(rootFolders []*radarr.RootFolder) (msgtext string) {
	maxLength := 0
	var text strings.Builder
	disks := make(map[string]string, len(rootFolders))
	for _, disk := range rootFolders {
		path := disk.Path
		freeSpace := disk.FreeSpace
		disks[fmt.Sprintf("%v:", path)] = Escape(ByteCountSI(freeSpace))

		length := len(path)
		if maxLength < length {
			maxLength = length
		}
	}

	formatter := fmt.Sprintf("`%%-%dv%%11v`\n", maxLength+1)
	for key, value := range disks {
		text.WriteString(fmt.Sprintf(formatter, key, value))
	}
	return text.String()
}

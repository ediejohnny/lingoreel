package subtitleformat

import "strings"

// splitLines splits text on LF, CRLF or CR.
func splitLines(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	return strings.Split(text, "\n")
}

func firstLine(text string) string {
	line, _, _ := strings.Cut(strings.ReplaceAll(text, "\r", "\n"), "\n")
	return line
}

func isBlank(line string) bool {
	return strings.TrimSpace(line) == ""
}

// endOfBlock returns the index of the first blank line at or after from.
func endOfBlock(lines []string, from int) int {
	for i := from; i < len(lines); i++ {
		if isBlank(lines[i]) {
			return i
		}
	}
	return len(lines)
}

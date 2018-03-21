package tar

import (
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
)

//https://msdn.microsoft.com/en-us/library/windows/desktop/aa365247(v=vs.85).aspx
var reservedNames = [...]string{"CON", "PRN", "AUX", "NUL", "COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9", "LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9"}

const reservedCharsRegex = `[<>:"\\|?*]` //NOTE: `/` is not included, files with this in the name will cause problems

func platformSanitize(pathElements []string) string {
	// first pass: scan and prefix reserved names CON -> _CON
	for i, pe := range pathElements {
		for _, rn := range reservedNames {
			if pe == rn {
				pathElements[i] = "_" + rn
				break
			}
		}
		pathElements[i] = strings.TrimRight(pe, ". ") //MSDN: Do not end a file or directory name with a space or a period
	}
	//second pass: scan and encode reserved characters ? -> %3F
	res := strings.Join(pathElements, `/`) // intentionally avoiding [file]path.Clean(), we want `\`'s intact
	re := regexp.MustCompile(reservedCharsRegex)
	illegalIndices := re.FindAllStringIndex(res, -1)

	if illegalIndices != nil {
		var lastIndex int
		var builder strings.Builder
		allocAssist := (len(res) - len(illegalIndices)) + (len(illegalIndices) * 3) // 3 = encoded length
		builder.Grow(allocAssist)

		for _, si := range illegalIndices {
			builder.WriteString(res[lastIndex:si[0]])              // append up to problem char
			builder.WriteString(url.QueryEscape(res[si[0]:si[1]])) // escape and append problem char
			lastIndex = si[1]
		}
		builder.WriteString(res[lastIndex:]) // append remainder
		res = builder.String()
	}

	return filepath.FromSlash(res)
}

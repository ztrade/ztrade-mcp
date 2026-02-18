package tools

import "unicode/utf8"

// truncateLinesByBytes returns a prefix of lines whose joined output (with "\n")
// is <= maxBytes. If the next line would exceed the limit, it is truncated to fit.
func truncateLinesByBytes(lines []string, maxBytes int) (out []string, truncated bool) {
	if maxBytes <= 0 {
		if len(lines) > 0 {
			return nil, true
		}
		return nil, false
	}

	used := 0
	for _, line := range lines {
		sep := 0
		if len(out) > 0 {
			sep = 1 // "\n"
		}

		need := sep + len(line)
		if used+need <= maxBytes {
			if sep > 0 {
				used += sep
			}
			out = append(out, line)
			used += len(line)
			continue
		}

		remaining := maxBytes - used - sep
		if remaining <= 0 {
			return out, true
		}
		if sep > 0 {
			used += sep
		}
		out = append(out, truncateStringUTF8(line, remaining))
		return out, true
	}

	return out, false
}

func truncateStringUTF8(s string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	if len(s) <= maxBytes {
		return s
	}
	b := s[:maxBytes]
	for len(b) > 0 && !utf8.ValidString(b) {
		b = b[:len(b)-1]
	}
	return b
}

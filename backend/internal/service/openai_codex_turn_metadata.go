package service

import (
	"encoding/json"
	"unicode/utf16"
)

// Turn metadata also travels in HTTP headers, including when embedded in WS JSON.
func marshalCodexTurnMetadata(metadata map[string]any) ([]byte, error) {
	raw, err := json.Marshal(metadata)
	if err != nil {
		return nil, err
	}
	for i, b := range raw {
		if b < 0x7f {
			continue
		}
		out := append(make([]byte, 0, len(raw)+16), raw[:i]...)
		appendEscape := func(r rune) {
			const hex = "0123456789abcdef"
			out = append(out, '\\', 'u', hex[r>>12&15], hex[r>>8&15], hex[r>>4&15], hex[r&15])
		}
		for _, r := range string(raw[i:]) {
			switch {
			case r < 0x7f:
				out = append(out, byte(r))
			case r <= 0xffff:
				appendEscape(r)
			default:
				high, low := utf16.EncodeRune(r)
				appendEscape(high)
				appendEscape(low)
			}
		}
		return out, nil
	}
	return raw, nil
}

package material

import (
	"bytes"
	"encoding/json"
	"io"
	"strconv"
	"unicode/utf8"
)

// CatalogBindingsMaxBytes bounds the deployment file before JSON decoding.
const CatalogBindingsMaxBytes = 64 * 1024

// ParseCatalogBindings parses deployment selection, not channel authorization.
// Errors deliberately contain neither raw JSON nor media identifiers.
func ParseCatalogBindings(data []byte) ([]CatalogBinding, error) {
	if len(data) > CatalogBindingsMaxBytes || !utf8.Valid(data) || !pairedUnicodeEscapes(data) {
		return nil, ErrInvalid
	}
	d := json.NewDecoder(bytes.NewReader(data))
	tok, err := d.Token()
	if err != nil || tok != json.Delim('[') {
		return nil, ErrInvalid
	}
	bindings := []CatalogBinding{}
	for d.More() {
		tok, err = d.Token()
		if err != nil || tok != json.Delim('{') {
			return nil, ErrInvalid
		}
		var b CatalogBinding
		fields := map[string]*string{"platform": &b.Platform, "type": &b.Type, "terminal": &b.Terminal, "scene": &b.Scene, "mediaId": &b.MediaID}
		for d.More() {
			tok, err = d.Token()
			name, ok := tok.(string)
			if err != nil || !ok {
				return nil, ErrInvalid
			}
			target, ok := fields[name]
			if !ok {
				return nil, ErrInvalid
			}
			tok, err = d.Token()
			value, ok := tok.(string)
			if err != nil || !ok {
				return nil, ErrInvalid
			}
			*target = value
			delete(fields, name)
		}
		tok, err = d.Token()
		if err != nil || tok != json.Delim('}') || len(fields) != 0 {
			return nil, ErrInvalid
		}
		bindings = append(bindings, b)
	}
	tok, err = d.Token()
	if err != nil || tok != json.Delim(']') {
		return nil, ErrInvalid
	}
	if _, err = d.Token(); err != io.EOF {
		return nil, ErrInvalid
	}
	if _, err = bindingMap(bindings); err != nil {
		return nil, ErrInvalid
	}
	return bindings, nil
}

// encoding/json silently replaces unpaired UTF-16 escapes. Deployment values
// must not change during decoding; ordinary escaped backslashes remain literal.
func pairedUnicodeEscapes(data []byte) bool {
	for i := 0; i < len(data); i++ {
		if data[i] != '\\' {
			continue
		}
		if i+1 >= len(data) {
			return false
		}
		if data[i+1] != 'u' {
			i++
			continue
		}
		if i+6 > len(data) {
			return false
		}
		v, err := strconv.ParseUint(string(data[i+2:i+6]), 16, 16)
		if err != nil || v >= 0xdc00 && v <= 0xdfff {
			return false
		}
		if v >= 0xd800 && v <= 0xdbff {
			if i+12 > len(data) || data[i+6] != '\\' || data[i+7] != 'u' {
				return false
			}
			low, err := strconv.ParseUint(string(data[i+8:i+12]), 16, 16)
			if err != nil || low < 0xdc00 || low > 0xdfff {
				return false
			}
			i += 6
		}
		i += 5
	}
	return true
}

func bindingMap(bindings []CatalogBinding) (map[bindingScope]string, error) {
	result := make(map[bindingScope]string, len(bindings))
	for _, binding := range bindings {
		if !platformType(binding.Platform, binding.Type) || !terminal(binding.Terminal) || !text(binding.Scene, 80, false) || !text(binding.MediaID, 128, false) {
			return nil, ErrInvalid
		}
		scope := bindingScope{binding.Platform, binding.Type, binding.Terminal, binding.Scene}
		if _, exists := result[scope]; exists {
			return nil, ErrInvalid
		}
		result[scope] = binding.MediaID
	}
	return result, nil
}

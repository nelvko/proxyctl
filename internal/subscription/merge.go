package subscription

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/nelvko/proxyctl/internal/kernel"
	"go.yaml.in/yaml/v3"
)

// mergeConfig applies object overlays in order. Arrays and scalar values,
// including null, replace the previous value; null is not a deletion operator.
func mergeConfig(format kernel.ConfigFormat, base []byte, overlays ...[]byte) ([]byte, error) {
	if len(overlays) == 0 {
		return base, nil
	}
	merged, err := decodeConfigObject(format, base)
	if err != nil {
		return nil, fmt.Errorf("source configuration: %w", err)
	}
	changed := false
	for i, overlay := range overlays {
		object, err := decodeConfigObject(format, overlay)
		if err != nil {
			return nil, fmt.Errorf("override %d: %w", i+1, err)
		}
		if len(object) != 0 {
			mergeObjects(merged, object)
			changed = true
		}
	}
	if !changed {
		return base, nil
	}
	if format == kernel.FormatSingBox {
		data, err := json.MarshalIndent(merged, "", "  ")
		if err != nil {
			return nil, errors.New("cannot encode merged JSON configuration")
		}
		return append(data, '\n'), nil
	}
	data, err := yaml.Marshal(merged)
	if err != nil {
		return nil, errors.New("cannot encode merged YAML configuration")
	}
	return data, nil
}

func mergeObjects(target, overlay map[string]any) {
	for key, value := range overlay {
		previous, previousObject := target[key].(map[string]any)
		next, nextObject := value.(map[string]any)
		if previousObject && nextObject {
			mergeObjects(previous, next)
		} else {
			target[key] = value
		}
	}
}

func decodeConfigObject(format kernel.ConfigFormat, data []byte) (map[string]any, error) {
	switch format {
	case kernel.FormatSingBox:
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.UseNumber()
		value, err := readJSONValue(decoder)
		if err != nil {
			return nil, err
		}
		if _, err := decoder.Token(); err != io.EOF {
			return nil, errors.New("expected one JSON document")
		}
		object, ok := value.(map[string]any)
		if !ok {
			return nil, errors.New("configuration root must be an object")
		}
		return object, nil
	case kernel.FormatClash:
		decoder := yaml.NewDecoder(bytes.NewReader(data))
		var document yaml.Node
		if err := decoder.Decode(&document); err != nil {
			return nil, errors.New("invalid YAML configuration")
		}
		var trailing yaml.Node
		if err := decoder.Decode(&trailing); err != io.EOF {
			return nil, errors.New("expected one YAML document")
		}
		if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
			return nil, errors.New("configuration root must be an object")
		}
		if err := validateYAMLKeys(&document, make(map[*yaml.Node]bool)); err != nil {
			return nil, err
		}
		var object map[string]any
		if err := document.Decode(&object); err != nil {
			return nil, errors.New("invalid YAML configuration")
		}
		return object, nil
	default:
		return nil, errors.New("unsupported configuration format")
	}
}

// Token decoding preserves JSON numbers and detects duplicate object keys,
// which decoding directly into a map would silently discard.
func readJSONValue(decoder *json.Decoder) (any, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, errors.New("invalid JSON configuration")
	}
	switch token {
	case json.Delim('{'):
		object := make(map[string]any)
		for decoder.More() {
			key, err := decoder.Token()
			if err != nil {
				return nil, errors.New("invalid JSON configuration")
			}
			name, ok := key.(string)
			if !ok {
				return nil, errors.New("object keys must be strings")
			}
			if _, exists := object[name]; exists {
				return nil, errors.New("duplicate object key")
			}
			object[name], err = readJSONValue(decoder)
			if err != nil {
				return nil, err
			}
		}
		if end, err := decoder.Token(); err != nil || end != json.Delim('}') {
			return nil, errors.New("invalid JSON configuration")
		}
		return object, nil
	case json.Delim('['):
		array := make([]any, 0)
		for decoder.More() {
			value, err := readJSONValue(decoder)
			if err != nil {
				return nil, err
			}
			array = append(array, value)
		}
		if end, err := decoder.Token(); err != nil || end != json.Delim(']') {
			return nil, errors.New("invalid JSON configuration")
		}
		return array, nil
	default:
		return token, nil
	}
}

func validateYAMLKeys(node *yaml.Node, visited map[*yaml.Node]bool) error {
	if visited[node] {
		return nil
	}
	visited[node] = true
	if node.Kind == yaml.AliasNode {
		return validateYAMLKeys(node.Alias, visited)
	}
	if node.Kind == yaml.MappingNode {
		keys := make(map[string]bool)
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i]
			if key.Kind != yaml.ScalarNode || (key.Tag != "!!str" && key.Tag != "!!merge") {
				return errors.New("object keys must be strings")
			}
			if keys[key.Value] {
				return errors.New("duplicate object key")
			}
			keys[key.Value] = true
		}
	}
	for _, child := range node.Content {
		if err := validateYAMLKeys(child, visited); err != nil {
			return err
		}
	}
	return nil
}

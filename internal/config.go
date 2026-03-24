package internal

import (
	"encoding/json"
	"my-sub-go/typedef"
	"os"
	"reflect"
	"strings"
)

func LoadConfig(path string) (*typedef.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var config typedef.Config
	err = json.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}

// Metadata

func ParseConfig(field reflect.StructField, parent string) *typedef.Metadata {
	// for GUI
	label := field.Tag.Get("label")
	if label == "" && field.Type.Kind() != reflect.Struct {
		return nil
	}

	meta := &typedef.Metadata{
		Name:        field.Name,
		Label:       label,
		Description: field.Tag.Get("description"),
		Type:        field.Tag.Get("type"),
		Placeholder: field.Tag.Get("placeholder"),
		SupportExt:  field.Tag.Get("support_ext"),
		//Options:     field.Tag.Get("options"),
		Group: field.Tag.Get("group"),
		Path:  field.Name,
	}

	if meta.Path != "" {
		meta.Path = parent + "." + meta.Path
	}
	if meta.Type != "" {
		// infer
		meta.Type = inferType(field.Type.Kind())
	}
	if field.Tag.Get("options") != "" {
		meta.Options = strings.Split(field.Tag.Get("options"), ",")
	}
	if meta.Type == "lang" {
		meta.Options = typedef.LangOptions
	}

	// inherit parent path: FFmpeg.BinaryPath
	if meta.Group == "" && parent != "" {
		parts := strings.Split(parent, ".")
		if len(parts) > 0 {
			meta.Group = parts[0]
		}
	}

	return meta
}

func inferType(kind reflect.Kind) string {
	switch kind {
	case reflect.Bool:
		return "bool"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return "int"
	case reflect.String:
		return "string"
	default:
		return "string"
	}
}

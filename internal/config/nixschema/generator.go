package nixschema

import (
	"fmt"
	"reflect"
	"strings"
)

type Field struct {
	Name        string
	Type        string
	Description string
	Default     string
	Required    bool
	JSONName    string
}

type Struct struct {
	Name       string
	NixDefName string
	Fields     []Field
}

type Generator struct {
	structs     map[string]*Struct
	processed   map[reflect.Type]bool
	typeAliases map[string]string
}

func NewGenerator() *Generator {
	return &Generator{
		structs:     make(map[string]*Struct),
		processed:   make(map[reflect.Type]bool),
		typeAliases: make(map[string]string),
	}
}

func (g *Generator) GetStructs() map[string]*Struct {
	return g.structs
}

func (g *Generator) GetNixDefName(goTypeName string) string {
	if alias, ok := g.typeAliases[goTypeName]; ok {
		return alias + "Def"
	}
	return goTypeName + "Def"
}

func (g *Generator) ParseStruct(t reflect.Type) error {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if t.Kind() != reflect.Struct {
		return fmt.Errorf("expected struct, got %s", t.Kind())
	}

	if g.processed[t] {
		return nil
	}

	g.processed[t] = true

	goTypeName := t.Name()
	if goTypeName == "" {
		return nil
	}

	nixDefName := goTypeName + "Def"

	if _, exists := g.structs[nixDefName]; exists {
		return nil
	}

	structInfo := &Struct{
		Name:       goTypeName,
		NixDefName: nixDefName,
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		if field.PkgPath != "" {
			continue
		}

		if field.Tag.Get("json") == "-" || field.Tag.Get("yaml") == "-" {
			continue
		}

		if field.Anonymous {
			embeddedType := field.Type
			if embeddedType.Kind() == reflect.Ptr {
				embeddedType = embeddedType.Elem()
			}
			if embeddedType.Kind() == reflect.Struct {
				if err := g.ParseStruct(embeddedType); err != nil {
					return err
				}
				embeddedDefName := embeddedType.Name() + "Def"
				if embeddedStruct, ok := g.structs[embeddedDefName]; ok {
					structInfo.Fields = append(structInfo.Fields, embeddedStruct.Fields...)
				}
			}
			continue
		}

		fieldInfo := g.parseField(field)
		if fieldInfo != nil {
			structInfo.Fields = append(structInfo.Fields, *fieldInfo)
		}
	}

	g.structs[nixDefName] = structInfo
	return nil
}

func (g *Generator) parseField(field reflect.StructField) *Field {
	jsonName := field.Tag.Get("json")
	if jsonName == "" {
		jsonName = strings.ToLower(field.Name)
	} else {
		if idx := strings.Index(jsonName, ","); idx != -1 {
			jsonName = jsonName[:idx]
		}
	}

	required := strings.Contains(field.Tag.Get("json"), "required") ||
		strings.Contains(field.Tag.Get("yaml"), "required")

	desc := field.Tag.Get("desc")
	defaultVal := field.Tag.Get("default")

	nixType := g.mapType(field.Type)

	fieldType := field.Type
	if fieldType.Kind() == reflect.Ptr {
		fieldType = fieldType.Elem()
	}

	if fieldType.Kind() == reflect.Struct && fieldType.Name() != "" {
		_ = g.ParseStruct(fieldType)
	}

	if field.Type.Kind() == reflect.Slice {
		elemType := field.Type.Elem()
		if elemType.Kind() == reflect.Ptr {
			elemType = elemType.Elem()
		}
		if elemType.Kind() == reflect.Struct && elemType.Name() != "" {
			_ = g.ParseStruct(elemType)
		}
	}

	return &Field{
		Name:        field.Name,
		Type:        nixType,
		Description: desc,
		Default:     defaultVal,
		Required:    required,
		JSONName:    jsonName,
	}
}

func (g *Generator) mapType(t reflect.Type) string {
	if t.Kind() == reflect.Ptr {
		return g.mapType(t.Elem())
	}

	switch t.Kind() {
	case reflect.String:
		return "types.str"
	case reflect.Bool:
		return "types.bool"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "types.int"
	case reflect.Float32, reflect.Float64:
		return "types.number"
	case reflect.Slice:
		elemType := t.Elem()
		if elemType.Kind() == reflect.Ptr {
			elemType = elemType.Elem()
		}

		if elemType.Kind() == reflect.String {
			return "(types.listOf types.str)"
		}
		if elemType.Kind() == reflect.Int || elemType.Kind() == reflect.Uint {
			return "(types.listOf types.int)"
		}
		if elemType.Kind() == reflect.Bool {
			return "(types.listOf types.bool)"
		}

		if elemType.Kind() == reflect.Struct {
			nixDefName := g.GetNixDefName(elemType.Name())
			return fmt.Sprintf("(types.listOf (types.submodule %s))", nixDefName)
		}

		return "(types.listOf types.attrs)"

	case reflect.Struct:
		if t.Name() == "Duration" {
			return "types.str"
		}
		nixDefName := g.GetNixDefName(t.Name())
		return fmt.Sprintf("(types.submodule %s)", nixDefName)

	default:
		return "types.attrs"
	}
}

func (g *Generator) formatDefaultValue(value, nixType string) string {
	// Handle empty defaults for list types
	if value == "" && (strings.HasPrefix(nixType, "(types.listOf") || strings.HasPrefix(nixType, "types.listOf")) {
		return "[]"
	}

	if strings.Contains(nixType, "types.str") && !strings.HasPrefix(value, `"`) {
		return fmt.Sprintf(`"%s"`, value)
	}

	if nixType == "types.bool" {
		if strings.ToLower(value) == "true" {
			return "true"
		}
		return "false"
	}

	if nixType == "types.int" || nixType == "types.number" {
		return value
	}

	if strings.HasPrefix(nixType, "(types.listOf") {
		return fmt.Sprintf("[ %s ]", value)
	}

	return value
}

func (g *Generator) getDefaultForType(nixType string) string {
	nixType = strings.TrimSpace(nixType)

	// Check list types first before checking for str/bool/etc (since list types contain those)
	if strings.HasPrefix(nixType, "(types.listOf") || strings.HasPrefix(nixType, "types.listOf") {
		return "[]"
	}
	if strings.HasPrefix(nixType, "(types.submodule") {
		return "{}"
	}
	if strings.HasPrefix(nixType, "types.attrsOf") {
		return "{}"
	}

	if strings.Contains(nixType, "types.str") {
		return `""`
	}
	if nixType == "types.bool" {
		return "false"
	}
	if nixType == "types.int" || nixType == "types.number" {
		return "0"
	}
	return "null"
}

func (g *Generator) GenerateNixModule() string {
	var sb strings.Builder

	sb.WriteString("# Panix Nix module options\n")
	sb.WriteString("# Generated from Go structs - DO NOT EDIT\n\n")
	sb.WriteString("{ lib, ... }:\n\n")
	sb.WriteString("with lib;\n\n")
	sb.WriteString("let\n")

	baseStructs := []string{"SSHClient", "KexecConfig", "PlainFileOrDirToTransfer", "NixConfig"}
	for _, name := range baseStructs {
		defName := name + "Def"
		if s, ok := g.structs[defName]; ok {
			g.generateSubmoduleDef(&sb, s)
		}
	}

	sb.WriteString("  # Bootstrap options for flags (subset)\n")
	sb.WriteString("  FlagsBootstrapDef = {\n")
	sb.WriteString("    options = {\n")
	sb.WriteString("      disable_auto = mkOption {\n")
	sb.WriteString("        type = types.bool;\n")
	sb.WriteString("        default = false;\n")
	sb.WriteString("      };\n")
	sb.WriteString("      disable_disko = mkOption {\n")
	sb.WriteString("        type = types.bool;\n")
	sb.WriteString("        default = false;\n")
	sb.WriteString("      };\n")
	sb.WriteString("    };\n")
	sb.WriteString("  };\n\n")

	if s, ok := g.structs["BootstrapDef"]; ok {
		g.generateSubmoduleDef(&sb, s)
	}

	if s, ok := g.structs["AttributesDef"]; ok {
		g.generateSubmoduleDef(&sb, s)
	}

	sb.WriteString("  # Submodules for hierarchical config structure\n")
	sb.WriteString("  MachineDef = {\n")
	sb.WriteString("    options = {\n")
	sb.WriteString("      name = mkOption {\n")
	sb.WriteString("        type = types.str;\n")
	sb.WriteString("        description = \"Machine name (auto-populated from attrset key)\";\n")
	sb.WriteString("      };\n")
	if s, ok := g.structs["AttributesDef"]; ok {
		for _, field := range s.Fields {
			g.generateFieldOption(&sb, field, 3)
		}
	}
	sb.WriteString("    };\n")
	sb.WriteString("  };\n\n")

	sb.WriteString("  ConfigurationDef = {\n")
	sb.WriteString("    options = {\n")
	sb.WriteString("      name = mkOption {\n")
	sb.WriteString("        type = types.str;\n")
	sb.WriteString("        description = \"Configuration name (auto-populated from attrset key)\";\n")
	sb.WriteString("      };\n")
	sb.WriteString("      flake_output = mkOption {\n")
	sb.WriteString("        type = types.str;\n")
	sb.WriteString("        description = \"Flake output attribute path\";\n")
	sb.WriteString("        default = \"\";\n")
	sb.WriteString("      };\n")
	sb.WriteString("      machines = mkOption {\n")
	sb.WriteString("        type = types.attrsOf (types.submodule MachineDef);\n")
	sb.WriteString("        default = {};\n")
	sb.WriteString("        description = \"Machine configurations\";\n")
	sb.WriteString("      };\n")
	if s, ok := g.structs["AttributesDef"]; ok {
		for _, field := range s.Fields {
			g.generateFieldOption(&sb, field, 3)
		}
	}
	sb.WriteString("    };\n")
	sb.WriteString("  };\n\n")

	sb.WriteString("  FlakeDef = {\n")
	sb.WriteString("    options = {\n")
	sb.WriteString("      name = mkOption {\n")
	sb.WriteString("        type = types.str;\n")
	sb.WriteString("        description = \"Flake name (auto-populated from attrset key)\";\n")
	sb.WriteString("      };\n")
	sb.WriteString("      url = mkOption {\n")
	sb.WriteString("        type = types.str;\n")
	sb.WriteString("        description = \"Flake URL or path\";\n")
	sb.WriteString("      };\n")
	sb.WriteString("      configurations = mkOption {\n")
	sb.WriteString("        type = types.attrsOf (types.submodule ConfigurationDef);\n")
	sb.WriteString("        default = {};\n")
	sb.WriteString("        description = \"Configurations for this flake\";\n")
	sb.WriteString("      };\n")
	if s, ok := g.structs["AttributesDef"]; ok {
		for _, field := range s.Fields {
			g.generateFieldOption(&sb, field, 3)
		}
	}
	sb.WriteString("    };\n")
	sb.WriteString("  };\n\n")

	if s, ok := g.structs["FlagsDef"]; ok {
		sb.WriteString("  FlagsDef = {\n")
		sb.WriteString("    options = {\n")
		for _, field := range s.Fields {
			if field.JSONName == "bootstrap" {
				sb.WriteString("      bootstrap = mkOption {\n")
				sb.WriteString("        type = types.submodule FlagsBootstrapDef;\n")
				sb.WriteString("        default = {};\n")
				sb.WriteString("      };\n")
			} else {
				g.generateFieldOption(&sb, field, 3)
			}
		}
		sb.WriteString("    };\n")
		sb.WriteString("  };\n\n")
	}

	sb.WriteString("in\n{\n")
	sb.WriteString("  options = {\n")
	sb.WriteString("    flags = mkOption {\n")
	sb.WriteString("      type = types.submodule FlagsDef;\n")
	sb.WriteString("      default = {};\n")
	sb.WriteString("      description = \"Global configuration flags\";\n")
	sb.WriteString("    };\n\n")
	sb.WriteString("    flakes = mkOption {\n")
	sb.WriteString("      type = types.attrsOf (types.submodule FlakeDef);\n")
	sb.WriteString("      default = {};\n")
	sb.WriteString("      description = \"Flake configurations\";\n")
	sb.WriteString("    };\n")
	sb.WriteString("  };\n")
	sb.WriteString("}\n")

	return sb.String()
}

func (g *Generator) generateSubmoduleDef(sb *strings.Builder, s *Struct) {
	sb.WriteString(fmt.Sprintf("  %s = {\n", s.NixDefName))
	sb.WriteString("    options = {\n")
	for _, field := range s.Fields {
		g.generateFieldOption(sb, field, 3)
	}
	sb.WriteString("    };\n")
	sb.WriteString("  };\n\n")
}

func (g *Generator) generateFieldOption(sb *strings.Builder, field Field, indent int) {
	indentStr := strings.Repeat("  ", indent)

	sb.WriteString(fmt.Sprintf("%s%s = mkOption {\n", indentStr, field.JSONName))
	sb.WriteString(fmt.Sprintf("%s  type = %s;\n", indentStr, field.Type))

	if field.Description != "" {
		desc := strings.ReplaceAll(field.Description, `"`, `\\"`)
		sb.WriteString(fmt.Sprintf("%s  description = \"%s\";\n", indentStr, desc))
	}

	if field.Default != "" {
		sb.WriteString(fmt.Sprintf("%s  default = %s;\n", indentStr, g.formatDefaultValue(field.Default, field.Type)))
	} else if !field.Required {
		sb.WriteString(fmt.Sprintf("%s  default = %s;\n", indentStr, g.getDefaultForType(field.Type)))
	}

	sb.WriteString(fmt.Sprintf("%s};\n", indentStr))
}

package template

import (
	"bytes"
	"os"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/lexer"
	"github.com/goccy/go-yaml/parser"
	"github.com/goccy/go-yaml/printer"
	"github.com/mihakrumpestar/panix/internal/config/filepermissions"
	"github.com/mihakrumpestar/panix/internal/pkg/yamlx"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

func EvalConfig(configPath string, outputPath string) error {
	//nolint:gosec // Config path is user-provided by design
	rawYAML, err := os.ReadFile(configPath)
	if err != nil {
		return errors.Wrapf(err, "failed reading config %s", configPath)
	}

	processedYAML, err := ProcessTemplate(rawYAML)
	if err != nil {
		return errors.Wrap(err, "failed to process templates")
	}

	orderedResult, err := parseAndOrderYAML(processedYAML)
	if err != nil {
		return err
	}

	var formatted bytes.Buffer

	err = yamlx.Encode(orderedResult, &formatted)
	if err != nil {
		return errors.Wrap(err, "failed to encode config")
	}

	return writeOutput(formatted.Bytes(), outputPath)
}

func parseAndOrderYAML(yamlData []byte) (yaml.MapSlice, error) {
	astFile, err := parser.ParseBytes(yamlData, parser.ParseComments)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse YAML AST")
	}

	if len(astFile.Docs) == 0 || astFile.Docs[0].Body == nil {
		return nil, errors.New("empty YAML document")
	}

	var decoded any

	err = yamlx.Decode(yamlData, &decoded)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode config")
	}

	return rebuildOrdered(astFile, decoded), nil
}

func writeOutput(data []byte, outputPath string) error {
	if outputPath == "-" {
		return writeToStdout(data)
	}

	if outputPath == "" {
		outputPath = "evaluated.yaml"
	}

	err := os.WriteFile(outputPath, data, filepermissions.DefaultFilePermissions)
	if err != nil {
		return errors.Wrap(err, "failed to write output file")
	}

	log.Info().Str("output", outputPath).Msg("Evaluated config written")

	return nil
}

func writeToStdout(data []byte) error {
	tokens := lexer.Tokenize(string(data))

	var printer_ printer.Printer

	printer_.Bool = func() *printer.Property { return &printer.Property{Prefix: "\x1b[33m", Suffix: "\x1b[0m"} }
	printer_.Number = func() *printer.Property { return &printer.Property{Prefix: "\x1b[32m", Suffix: "\x1b[0m"} }
	printer_.String = func() *printer.Property { return &printer.Property{Prefix: "\x1b[32m", Suffix: "\x1b[0m"} }
	printer_.MapKey = func() *printer.Property { return &printer.Property{Prefix: "\x1b[36m", Suffix: "\x1b[0m"} }
	_, err := os.Stdout.Write([]byte(printer_.PrintTokens(tokens)))

	return errors.Wrap(err, "failed to write to stdout")
}

func rebuildOrdered(astFile *ast.File, decoded any) yaml.MapSlice {
	mappingNode, ok := astFile.Docs[0].Body.(*ast.MappingNode)
	if !ok {
		return nil
	}

	decodedMap := toMap(decoded)
	if decodedMap == nil {
		return nil
	}

	var result yaml.MapSlice

	for _, mappingValue := range mappingNode.Values {
		key := mappingValue.Key.GetToken().Value
		if key == "flags" || key == "fleet" { // This prevents anchor definitions to remain in output
			var val any

			val, ok = decodedMap[key]
			if ok {
				result = append(result, yaml.MapItem{
					Key:   key,
					Value: rebuildOrderedRecursive(mappingValue.Value, val),
				})
			}
		}
	}

	return result
}

func toMap(v any) map[string]any {
	switch m := v.(type) {
	case map[string]any:
		return m
	case yaml.MapSlice:
		result := make(map[string]any, len(m))
		for _, item := range m {
			key, ok := item.Key.(string)
			if !ok {
				continue
			}

			result[key] = item.Value
		}

		return result
	default:
		return nil
	}
}

func rebuildOrderedRecursive(astNode ast.Node, decoded any) any {
	decodedMap := toMap(decoded)
	if decodedMap != nil {
		return rebuildOrderedMap(astNode, decodedMap)
	}

	_, ok := decoded.([]any)
	if ok {
		return decoded
	}

	return decoded
}

func rebuildOrderedMap(astNode ast.Node, decodedVal map[string]any) yaml.MapSlice {
	mappingNode, ok := astNode.(*ast.MappingNode)
	if !ok {
		return mapToMapSlice(decodedVal)
	}

	seen := make(map[string]bool)

	var result yaml.MapSlice

	for _, mappingValue := range mappingNode.Values {
		key := mappingValue.Key.GetToken().Value
		if key == "<<" {
			continue
		}

		var val any

		val, ok = decodedVal[key]
		if ok {
			result = append(result, yaml.MapItem{
				Key:   key,
				Value: rebuildOrderedRecursive(mappingValue.Value, val),
			})
			seen[key] = true
		}
	}

	for k, v := range decodedVal {
		if !seen[k] {
			result = append(result, yaml.MapItem{Key: k, Value: v})
		}
	}

	return result
}

func mapToMapSlice(m map[string]any) yaml.MapSlice {
	result := make(yaml.MapSlice, len(m))

	i := 0
	for k, v := range m {
		result[i] = yaml.MapItem{Key: k, Value: v}
		i++
	}

	return result
}

package yamlx

import (
	"io"
	"os"

	"github.com/goccy/go-yaml/lexer"
	"github.com/goccy/go-yaml/printer"
	"github.com/mattn/go-colorable"
	"github.com/pkg/errors"
)

const sgrReset = "\x1b[0m"

type colorConfig struct {
	Bool    string
	Number  string
	String  string
	MapKey  string
	Anchor  string
	Alias   string
	Comment string
}

var defaultColors = colorConfig{
	Bool:    "\x1b[38;5;141m",
	Number:  "\x1b[38;5;215m",
	String:  "\x1b[38;5;113m",
	MapKey:  "\x1b[38;5;117m",
	Anchor:  "\x1b[38;5;229m",
	Alias:   "\x1b[38;5;229m",
	Comment: "\x1b[38;5;61m",
}

func (c colorConfig) apply(printerI *printer.Printer) {
	set := func(color string, target *printer.PrintFunc) {
		if color != "" {
			prop := &printer.Property{Prefix: color, Suffix: sgrReset}
			*target = func() *printer.Property { return prop }
		}
	}

	set(c.Bool, &printerI.Bool)
	set(c.Number, &printerI.Number)
	set(c.String, &printerI.String)
	set(c.MapKey, &printerI.MapKey)
	set(c.Anchor, &printerI.Anchor)
	set(c.Alias, &printerI.Alias)
	set(c.Comment, &printerI.Comment)
}

func printColorized(yamlData []byte, writer io.Writer) error {
	tokens := lexer.Tokenize(string(yamlData))

	var printer printer.Printer
	defaultColors.apply(&printer)

	f, ok := writer.(*os.File)
	if ok {
		writer = colorable.NewColorable(f)
	}

	_, err := writer.Write([]byte(printer.PrintTokens(tokens) + "\n"))

	return errors.Wrap(err, "failed to write colorized YAML")
}

package flags

import (
	"os"
	"reflect"
	"strings"
	"unicode"
	"unicode/utf8"

	"dario.cat/mergo"
	"github.com/pkg/errors"
)

// Source precedence for the effective flags, highest first: explicit CLI
// flags, explicitly set PANIX_* env vars, config file values, kong defaults.
// ExplicitMask derives the explicit set from the first two sources;
// ApplySources folds parsed values into the config values by that precedence.

// KongFlagName converts a Go struct field name into the kong flag name (and the
// PANIX_* env var name). It is a byte-exact port of kong's default namer,
// registered with kong.FlagNamer in cmd/panix/main.go as the single source of
// truth for names.
func KongFlagName(fieldName string) string {
	return strings.ToLower(strings.Join(camelCaseWords(fieldName), "-"))
}

// Character classes camelCaseWords splits identifiers on.
const (
	classLower = 1
	classUpper = 2
	classDigit = 3
	classOther = 4
)

// runeClass classifies a rune into one of the camelCaseWords split classes.
func runeClass(char rune) int {
	switch {
	case unicode.IsLower(char):
		return classLower
	case unicode.IsUpper(char):
		return classUpper
	case unicode.IsDigit(char):
		return classDigit
	default:
		return classOther
	}
}

// camelCaseWords splits a Go identifier like the splitter behind kong's dashed
// flag names (github.com/fatih/camelcase, MIT). Kept byte-identical on purpose.
func camelCaseWords(src string) []string {
	// Don't split invalid utf8.
	if !utf8.ValidString(src) {
		return []string{src}
	}

	var words [][]rune

	var lastClass int

	// Split into fields based on class of unicode character.
	for _, char := range src {
		class := runeClass(char)
		if class == lastClass {
			words[len(words)-1] = append(words[len(words)-1], char)
		} else {
			words = append(words, []rune{char})
		}

		lastClass = class
	}

	// Handle upper case to lower case sequences, e.g.
	// "PDFL", "oader" -> "PDF", "Loader".
	for idx := range len(words) - 1 {
		if unicode.IsUpper(words[idx][0]) && unicode.IsLower(words[idx+1][0]) {
			words[idx+1] = append([]rune{words[idx][len(words[idx])-1]}, words[idx+1]...)
			words[idx] = words[idx][:len(words[idx])-1]
		}
	}

	entries := []string{}

	for _, word := range words {
		if len(word) > 0 {
			entries = append(entries, string(word))
		}
	}

	return entries
}

// leafField is one kong flag leaf of Flags: its reflect index path, whether it
// takes no value (a bool) and its short:"" rune (0 when it has no short form).
type leafField struct {
	index  []int
	isBool bool
	short  rune
}

// leafFields maps every kong flag name of Flags to its leaf metadata. The walk
// mirrors kong's flattening: anonymous embeds and embed:"" fields are
// transparent and accumulate their prefix:"" tag, explicit name:"" tags win and
// every other exported field is a leaf. Boolness comes from the field kind; no
// Flags leaf implements kong.BoolMapperValue (TestNoBoolMapperLeaf guards this).
func leafFields(typ reflect.Type) map[string]leafField {
	fields := map[string]leafField{}

	walkLeafFields(typ, nil, "", fields)

	return fields
}

// leafFieldNames is the index-only view of leafFields, used by tests that do
// not need bool or short metadata.
func leafFieldNames(typ reflect.Type) map[string][]int {
	fields := leafFields(typ)
	names := make(map[string][]int, len(fields))

	for name, field := range fields {
		names[name] = field.index
	}

	return names
}

func walkLeafFields(typ reflect.Type, index []int, prefix string, fields map[string]leafField) {
	for fieldIdx := range typ.NumField() {
		field := typ.Field(fieldIdx)

		if field.Tag.Get("kong") == "-" || !field.IsExported() {
			continue
		}

		fieldIndex := append(append([]int(nil), index...), fieldIdx)
		fieldPrefix := prefix + field.Tag.Get("prefix")

		if field.Anonymous || hasEmbedTag(field.Tag) {
			walkLeafFields(field.Type, fieldIndex, fieldPrefix, fields)

			continue
		}

		name := field.Tag.Get("name")
		if name == "" {
			name = KongFlagName(field.Name)
		}

		fields[fieldPrefix+name] = leafField{
			index:  fieldIndex,
			isBool: field.Type.Kind() == reflect.Bool,
			short:  shortRune(field.Tag),
		}
	}
}

func hasEmbedTag(tag reflect.StructTag) bool {
	_, ok := tag.Lookup("embed")

	return ok
}

// shortRune returns the rune declared by a leaf's short:"" tag, or 0 when absent.
// kong reads the same tag with Tag.GetRune.
func shortRune(tag reflect.StructTag) rune {
	for _, char := range tag.Get("short") {
		return char
	}

	return 0
}

// EnvPrefix is the prefix kong.DefaultEnvars is configured with in
// cmd/panix/main.go. It is the single source of truth for the PANIX_* namespace.
const EnvPrefix = "PANIX"

// envarName returns the PANIX_* environment variable kong.DefaultEnvars
// generates for a flag name. The real leaf names only contain lower-case words
// separated by "." and "-", so this matches kong without its camelCase pass.
func envarName(flagName string) string {
	replacer := strings.NewReplacer("-", "_", ".", "_")

	return EnvPrefix + "_" + strings.ToUpper(replacer.Replace(flagName))
}

// ExplicitMask returns the kong flag names the user explicitly set on the command
// line or through a PANIX_* env var, restricted to the flags the running command
// declares. cmd is the command struct value or pointer whose Run is executing;
// its names come from the same struct-tag walker as Flags. Scoping matters:
// without it an env var such as PANIX_TUI_THEME would wipe a config value under a
// command that never reads the flag.
//
// Provenance comes from the raw sources, not kong's parse trace: argv is scanned
// against the scoped names (see scanArgv) and every scoped leaf whose env var is
// set, even to an empty value, is explicit, matching kong's Value.Reset.
func ExplicitMask(args []string, cmd any) map[string]bool {
	leaves := scopeLeaves(leafFields(reflect.TypeFor[Flags]()), cmd)
	explicit := map[string]bool{}

	scanArgv(args, leaves, explicit)

	for name := range leaves {
		if _, ok := os.LookupEnv(envarName(name)); ok {
			explicit[name] = true
		}
	}

	return explicit
}

// scopeLeaves restricts the Flags leaves to the kong flag names cmd declares.
// A nil or non-struct cmd keeps the full Flags leaf set.
func scopeLeaves(leaves map[string]leafField, cmd any) map[string]leafField {
	typ := commandType(cmd)
	if typ == nil {
		return leaves
	}

	names := leafFieldNames(typ)
	scoped := make(map[string]leafField, len(names))

	for name, field := range leaves {
		if _, ok := names[name]; ok {
			scoped[name] = field
		}
	}

	return scoped
}

// commandType returns the struct type behind a command value or pointer, or nil
// when cmd is nil or not a struct.
func commandType(cmd any) reflect.Type {
	typ := reflect.TypeOf(cmd)
	if typ == nil {
		return nil
	}

	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}

	if typ.Kind() != reflect.Struct {
		return nil
	}

	return typ
}

// scanArgv records every leaf flag that appears in args, mirroring the subset of
// kong's tokenizer that determines provenance, not values: "--" ends scanning;
// "--name"/"--name=value" match long names, with non-bool leaves consuming a
// following token when there is no "=value"; "-c"/"-cvalue"/"-c=value" and
// combined bool shorts match short tags; unknown names and positionals are
// skipped. kong keeps the "=" in "-c=value" values, which is irrelevant here.
func scanArgv(args []string, leaves map[string]leafField, explicit map[string]bool) {
	shorts := map[rune]string{}

	for name, field := range leaves {
		if field.short != 0 {
			shorts[field.short] = name
		}
	}

	for index := 0; index < len(args); index++ {
		arg := args[index]

		switch {
		case arg == "--":
			return
		case strings.HasPrefix(arg, "--"):
			index = scanLongFlag(args, index, leaves, explicit)
		case len(arg) > 1 && arg[0] == '-':
			index = scanShortFlag(args, index, shorts, leaves, explicit)
		}
	}
}

// scanLongFlag records the leaf a long token names and returns the index of the
// last token it consumed, so a non-bool leaf can absorb its value.
func scanLongFlag(args []string, index int, leaves map[string]leafField, explicit map[string]bool) int {
	name, _, hasValue := strings.Cut(args[index][2:], "=")

	field, ok := leaves[name]
	if !ok {
		return index
	}

	explicit[name] = true

	if !field.isBool && !hasValue {
		return index + 1
	}

	return index
}

// scanShortFlag records the leaf shorts in the token at args[index] and returns
// the index of the last token it consumed, so a non-bool short can absorb its value.
func scanShortFlag(
	args []string,
	index int,
	shorts map[rune]string,
	leaves map[string]leafField,
	explicit map[string]bool,
) int {
	body := []rune(args[index][1:])

	for runeIndex, short := range body {
		name, ok := shorts[short]
		if !ok {
			// Unknown short: kong rejects the token, so stop reading it.
			return index
		}

		explicit[name] = true

		if leaves[name].isBool {
			continue
		}

		// A non-bool short takes the rest of the token as its value; when the
		// token ends here, the value is the next token.
		if runeIndex == len(body)-1 {
			return index + 1
		}

		return index
	}

	return index
}

// ApplySources folds the parsed source values (explicit CLI flags and env vars
// plus kong defaults) into f, which holds the config file values, honoring the
// package precedence. Step one fills every empty field from the parsed values, so
// kong defaults and non-explicit env values only fill gaps. Step two rewrites
// every explicitly set field unconditionally, so explicit CLI/env values win even
// when they are the Go zero value (--flag=false, --count=0, empty string).
func (f *Flags) ApplySources(parsed Flags, explicit map[string]bool) error {
	err := mergo.Merge(f, parsed)
	if err != nil {
		return errors.Wrap(err, "failed to merge flags")
	}

	flagsValue := reflect.ValueOf(f).Elem()
	parsedValue := reflect.ValueOf(parsed)

	for name, field := range leafFields(reflect.TypeFor[Flags]()) {
		if !explicit[name] {
			continue
		}

		flagsValue.FieldByIndex(field.index).Set(parsedValue.FieldByIndex(field.index))
	}

	return nil
}

// unsetEnvars removes every PANIX_* environment variable and returns a function
// restoring the previous state. Source precedence must be exercised against a
// clean environment: kong resolves PANIX_* at parse time and ExplicitMask treats
// any set envar, even an empty one, as explicit.
func unsetEnvars() func() {
	type envar struct {
		name  string
		value string
	}

	var removed []envar

	prefix := EnvPrefix + "_"

	for _, item := range os.Environ() {
		name, value, ok := strings.Cut(item, "=")
		if !ok || !strings.HasPrefix(name, prefix) {
			continue
		}

		removed = append(removed, envar{name: name, value: value})

		_ = os.Unsetenv(name)
	}

	return func() {
		for _, item := range removed {
			_ = os.Setenv(item.name, item.value)
		}
	}
}

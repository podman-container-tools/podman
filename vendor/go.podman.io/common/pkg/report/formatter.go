package report

import (
	"io"
	"maps"
	"strings"
	"text/tabwriter"
	"text/template"
)

// Flusher is the interface that wraps the Flush method.
type Flusher interface {
	Flush() error
}

// NopFlusher represents a type which flush operation is nop.
type NopFlusher struct{}

// Flush is a nop operation.
func (f *NopFlusher) Flush() error {
	return nil
}

type Origin int

const (
	OriginUnknown Origin = iota
	OriginPodman
	OriginUser
)

func (o Origin) String() string {
	switch o {
	case OriginPodman:
		return "OriginPodman"
	case OriginUser:
		return "OriginUser"
	default:
		return "OriginUnknown"
	}
}

// Formatter holds the configured Writer and parsed Template, additional state fields are
// maintained to assist in the podman command report writing.
type Formatter struct {
	Origin        Origin             // Source of go template. OriginUser or OriginPodman
	RenderHeaders bool               // Hint, default behavior for given template is to include headers
	RenderTable   bool               // Does template have "table" keyword
	flusher       Flusher            // Flush any buffered formatted output
	template      *template.Template // Go text/template for formatting output
	text          string             // value of canonical template after processing
	writer        io.Writer          // Destination for formatted output
}

// stringsCutPrefix is equivalent to Go 1.20’s strings.CutPrefix.
// Replace this function with a direct call to the standard library after we update to Go 1.20.
func stringsCutPrefix(s, prefix string) (string, bool) {
	if !strings.HasPrefix(s, prefix) {
		return s, false
	}
	return s[len(prefix):], true
}

// Parse parses golang template returning a formatter
//
//   - OriginPodman implies text is a template from podman code. Output will
//     be filtered through a tabwriter.
//
//   - OriginUser implies text is a template from a user. If template includes
//     keyword "table" output will be filtered through a tabwriter.
func (f *Formatter) Parse(origin Origin, text string) (*Formatter, error) {
	f.Origin = origin

	// docker tries to be smart and replaces \n with the actual newline character.
	// For compat we do the same but this will break formats such as '{{printf "\n"}}'
	// To be backwards compatible with the previous behavior we try to replace and
	// parse the template. If it fails use the original text and parse again.
	var normText string
	textWithoutTable, hasTable := stringsCutPrefix(text, "table ")
	switch {
	case hasTable:
		f.RenderTable = true
		normText = "{{range .}}" + NormalizeFormat(text) + "{{end -}}"
		text = "{{range .}}" + textWithoutTable + "{{end -}}"
	case OriginUser == origin:
		normText = EnforceRange(NormalizeFormat(text))
		text = EnforceRange(text)
	default:
		normText = NormalizeFormat(text)
	}

	if f.RenderTable || origin == OriginPodman {
		tw := tabwriter.NewWriter(f.writer, 12, 2, 2, ' ', tabwriter.StripEscape)
		f.writer = tw
		f.flusher = tw
		f.RenderHeaders = true
	}

	tmpl, err := f.template.Funcs(template.FuncMap(DefaultFuncs)).Parse(normText)
	if err != nil {
		tmpl, err = f.template.Funcs(template.FuncMap(DefaultFuncs)).Parse(text)
		f.template = tmpl
		f.text = text
		return f, err
	}
	f.text = normText
	f.template = tmpl
	return f, nil
}

// Funcs adds the elements of the argument map to the template's function map.
// A default template function will be replaced if there is a key collision.
func (f *Formatter) Funcs(funcMap template.FuncMap) *Formatter {
	m := make(template.FuncMap, len(DefaultFuncs)+len(funcMap))
	maps.Copy(m, DefaultFuncs)
	maps.Copy(m, funcMap)
	f.template = f.template.Funcs(funcMap)
	return f
}

// Init either resets the given tabwriter with new values or wraps w in tabwriter with given values.
func (f *Formatter) Init(w io.Writer, minwidth, tabwidth, padding int, padchar byte, flags uint) *Formatter {
	flags |= tabwriter.StripEscape

	if tw, ok := f.writer.(*tabwriter.Writer); ok {
		tw = tw.Init(w, minwidth, tabwidth, padding, padchar, flags)
		f.writer = tw
		f.flusher = tw
	} else {
		tw = tabwriter.NewWriter(w, minwidth, tabwidth, padding, padchar, flags)
		f.writer = tw
		f.flusher = tw
	}
	return f
}

// Execute applies a parsed template to the specified data object,
// and writes the output to Formatter.Writer.
func (f *Formatter) Execute(data any) error {
	return f.template.Execute(f.writer, data)
}

// Flush should be called after the last call to Write to ensure
// that any data buffered in the Formatter is written to output. Any
// incomplete escape sequence at the end is considered
// complete for formatting purposes.
func (f *Formatter) Flush() error {
	// Indirection is required here to prevent caller from having to know when
	// value of Flusher may be changed.
	return f.flusher.Flush()
}

// Writer returns the embedded io.Writer from Formatter.
func (f *Formatter) Writer() io.Writer {
	return f.writer
}

// New allocates a new, undefined Formatter with the given name and Writer.
func New(output io.Writer, name string) *Formatter {
	f := new(Formatter)

	f.flusher = new(NopFlusher)
	if flusher, ok := output.(Flusher); ok {
		f.flusher = flusher
	}

	f.template = template.New(name)
	f.writer = output
	return f
}

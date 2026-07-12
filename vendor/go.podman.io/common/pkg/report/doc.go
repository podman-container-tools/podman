/*
Package report provides helper structs/methods/funcs for formatting output

# Examples

To format output for an array of structs:

ExamplePodman:

	headers := report.Headers(struct {
		ID string
	}{}, nil)

	f := report.New(os.Stdout, "Command Name")
	f, _ := f.Parse(report.OriginPodman, "{{range .}}{{.ID}}{{end}}")
	defer f.Flush()

	if f.RenderHeaders {
		f.Execute(headers)
	}
	f.Execute( map[string]string{
		"ID":"fa85da03b40141899f3af3de6d27852b",
	})

	// Output:
	// ID
	// fa85da03b40141899f3af3de6d27852b

ExampleUser:

	headers := report.Headers(struct {
		CID string
	}{}, map[string]string{"CID":"ID"})

	f, _ := report.New(os.Stdout, "Command Name").Parse(report.OriginUser, "table {{.CID}}")
	defer f.Flush()

	if f.RenderHeaders {
		t.Execute(t, headers)
	}
	t.Execute(t,map[string]string{
		"CID":"fa85da03b40141899f3af3de6d27852b",
	})

	// Output:
	// ID
	// fa85da03b40141899f3af3de6d27852b

Helpers:

	if report.IsJSON(cmd.Flag("format").Value.String()) {
		... process JSON and output
	}

	if report.HasTable(cmd.Flag("format").Value.String()) {
		... "table" keyword prefix in format text
	}

# Template Functions

The following template functions are added to the template when parsed:
  - join  strings.Join, {{join .Field separator}}
  - json encode field as JSON {{ json .Field }}
  - lower strings.ToLower {{ .Field | lower }}
  - pad add spaces as prefix and suffix {{ pad . 2 2 }}
  - split strings.Split {{ .Field | split }}
  - title strings.Title {{ .Field | title }}
  - truncate limit field length {{ truncate . 10 }}
  - upper strings.ToUpper {{ .Field | upper }}

report.Funcs() may be used to add additional template functions.
Adding an existing function will replace that function for the life of that template.

Note: Your code should not ignore errors
*/
package report

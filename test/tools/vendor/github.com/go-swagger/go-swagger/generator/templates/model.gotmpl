{{ template "header" . }}
{{- if .IncludeModel }}
  {{- if .IsExported }}
// {{ pascalize .Name }} {{ template "docstring" . }}
    {{- template "annotations" . }}
  {{- end }}
  {{- template "schema" . }}
{{- end }}

{{ range .ExtraSchemas }}
  {{- if .IncludeModel }}
    {{- if .IsExported }}
// {{ pascalize .Name }} {{ template "docstring" . }}
      {{- template "annotations" . }}
    {{- end }}
    {{- template "schema" . }}
  {{- end }}
{{- end }}
{{- define "annotations" }}{{/* annotations to generate spec from source */}}
  {{- if not .IsBaseType }}
//
// swagger:model {{ .Name }}
  {{- else }}
//
// swagger:discriminator {{ .Name }} {{ .DiscriminatorField }}
  {{- end }}
{{- end }}

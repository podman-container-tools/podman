####> This option file is used in:
####>   podman podman-container.unit.5.md.in, create, exec, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `EnvironmentFile=file`
<< else >>
#### **--env-file**=*file*
<< endif >>

Read in a line-delimited file of environment variables. Leading spaces (0x20)
and tabs (0x09) are removed from each line first. The line is then one of:

- An empty line, or one starting with `#`: skipped.
- `KEY=value`: split at the first `=`. Key and value are used as-is (whitespace
  around `=` and at the end of the line, quotes and `#` are kept), with no
  escaping or variable expansion.
- `KEY`: the value is taken from the host environment. Not set if missing on
  the host.
- `KEY*`: all host environment variables starting with `KEY` are added. A bare
  `*` adds the whole host environment.
- `=value`, with an empty key: invalid, the command fails.

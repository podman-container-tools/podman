% podman-config-inspect 1

## NAME
podman\-config\-inspect - Display detailed information on one or more configs

## SYNOPSIS
**podman config inspect** [*options*] *config* [...]

## DESCRIPTION

Inspects the specified config.

By default, this renders all results in a JSON array. If a format is specified, the given template is executed for each result.
Configs can be queried individually by providing their full name or a unique partial name.

## OPTIONS

#### **--format**, **-f**=*format*

Format config output using Go template.

| **Placeholder**          | **Description**                                                   |
|--------------------------|-------------------------------------------------------------------|
| .ConfigData              | Config Data (Displayed only with --showconfig option)             |
| .CreatedAt ...           | When config was created (relative timestamp, human-readable)      |
| .ID                      | ID of config                                                      |
| .Spec ...                | Details of config                                                 |
| .Spec.Driver ...         | Driver info                                                       |
| .Spec.Driver.Name        | Driver name (string)                                              |
| .Spec.Driver.Options ... | Driver options (map of driver-specific options)                   |
| .Spec.Labels ...         | Labels for this config                                            |
| .Spec.Name               | Name of config                                                    |
| .UpdatedAt ...           | When config was last updated (relative timestamp, human-readable) |

#### **--help**

Print usage statement.

#### **--pretty**

Print inspect output in human-readable format. Ignores fields from **--format**.

#### **--showconfig**

Display config data

## EXAMPLES

Inspect the config myconfig.
```
$ podman config inspect myconfig
```

Inspect the config myconfig and print it in a human readable format instead of JSON, with the default fields.

```
$ podman config inspect --pretty myconfig
```

Inspect the config myconfig and display the Name and Labels field.
```
$ podman config inspect --format "{{.Spec.Name}} {{.Spec.Labels}}" myconfig
```

Inspect the config myconfig and display the Name and ConfigData fields. Note this will display the config data to the screen.
```
$ podman config inspect --showconfig --format "{{.Spec.Name}} {{.ConfigData}}" myconfig
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-config(1)](podman-config.1.md)**

## HISTORY
January 2021, Originally compiled by Ashley Cui <acui@redhat.com>

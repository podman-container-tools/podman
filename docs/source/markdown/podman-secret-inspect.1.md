% podman-secret-inspect 1

## NAME
podman\-secret\-inspect - Display detailed information on one or more secrets

## SYNOPSIS
**podman secret inspect** [*options*] *secret* [...]

## DESCRIPTION

Inspects the specified secret.

By default, this renders all results in a JSON array. If a format is specified, the given template is executed for each result.
Secrets can be queried individually by providing their full name or a unique partial name.

## OPTIONS

#### **--format**, **-f**=*format*

Format secret output using Go template.

| **Placeholder**          | **Description**                                                   |
|--------------------------|-------------------------------------------------------------------|
| .CreatedAt ...           | When secret was created (relative timestamp, human-readable)      |
| .ID                      | ID of secret                                                      |
| .SecretData              | Secret Data (Displayed only with --showsecret option)		       |
| .Spec ...                | Details of secret                                                 |
| .Spec.Driver ...         | Driver info                                                       |
| .Spec.Driver.Name        | Driver name (string)                                              |
| .Spec.Driver.Options ... | Driver options (map of driver-specific options)                   |
| .Spec.Labels ...         | Labels for this secret                                            |
| .Spec.Name               | Name of secret                                                    |
| .UpdatedAt ...           | When secret was last updated (relative timestamp, human-readable) |

#### **--help**

Print usage statement.

#### **--pretty**

Print inspect output in human-readable format. Ignores fields from **--format**.

#### **--showsecret**

Display secret data

## EXAMPLES

Inspect the secret mysecret.
```
$ podman secret inspect mysecret
```

Inspect the secret mysecret and print it in a human readable format instead of JSON, with the default fields.

```
$ podman secret inspect --pretty mysecret
```

Inspect the secret mysecret and display the Name and Labels field.
```
$ podman secret inspect --format "{{.Spec.Name}} {{.Spec.Labels}}" mysecret
```

Inspect the secret mysecret and display the Name and SecretData fields. Note this will display the secret data to the screen.
```
$ podman secret inspect --showsecret --format "{{.Spec.Name}} {{.SecretData}}" mysecret
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-secret(1)](podman-secret.1.md)**

## HISTORY
January 2021, Originally compiled by Ashley Cui <acui@redhat.com>

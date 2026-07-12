% podman-command 1

## NAME
podman\-command - short description

## SYNOPSIS
(Shows the command structure. If the command can be written in two different ways, both of them have to be shown.\
Many manpages include the OPTIONS **--all**, **-a** and/or **--latest**, **-l**. In this case, there is no `container name` or `ID` needed after the initial command. Because most of the other OPTIONS still need the `container name` or ` ID`, it is defined that the *container* argument in the command should **not** be put in brackets. It should also be noted in the *IMPORTANT* section in the description of the OPTION with the following sentence: *IMPORTANT: This OPTION does not need a container name or ID as input argument.*.)

**podman command** [*optional*] *mandatory value*

**podman subcommand command** [*optional*] *mandatory value*

(If there is the possibility to chose between two or more mandatory command values. There should also always be a space before and after a vertical bar to ensure better readability.)

**podman command** [*optional*] *value1* | *value2*

**podman subcommand command** [*optional*] *value1* | *value2*

(If an optional value follows a mandatory one.)

**podman command** [*optional*] *value1* | *value2* [*optional*]

**podman subcommand command** [*optional*] *value1* | *value2* [*optional*]

(If the command accepts an infinite number of values.)

**podman command** [*optional*] *value* [*value* ...]

**podman subcommand command** [*optional*] *value* [*value* ...]

## DESCRIPTION
**podman command** is always the beginning of the DESCRIPTION section. Putting the command as the first part of the DESCRIPTION ensures uniformity. All commands mentioned in the text retain their appearance and form.\
Example for the first sentence: **podman command** is an example command.

Commands or files that are quoted from other podman manpages or podman repositories have to be linked to those. Non-podman commands are not to be linked.\
Example sentence: Use **[podman-run](podman-run.1.md)** or **[containers.conf(5)](https://github.com/containers/container-libs/blob/main/common/docs/containers.conf.5.md)** for the problem.

It should also be specified if the command can only be run as root. In addition, it should be described when a command, OPTION, or other content cannot be executed with the remote client or in combination with other commands, OPTIONS, or content. In this case, the following sentence is put at the end of a command, OPTION, or content:\
*IMPORTANT: This command/OPTION/content is not available with the command/OPTION/content/on the remote Podman client.*\
For a command, this should be done in the DESCRIPTION section. For the OPTIONS, it should be done in the DESCRIPTION of the specified OPTION.

Do not use pronouns in the man pages, especially the word `you`.

There should be **no** new line after H2-headers (`##`).

## OPTIONS
All flags are referred to as OPTIONS. The term flags should not be used. All OPTIONS are listed in this section. OPTIONS that appear in descriptions of other OPTIONS and sections retain their appearance, for example: **--exit**.

OPTIONS that are quoted from other podman manpages or podman repositories have to be linked to those.\
Example sentence: Use **[podman-generate-systemd --new](./source/markdown/podman-generate-systemd.1.md#--new)** for the problem.

 Each OPTION should be explained to the fullest extent below the OPTION itself. Each OPTION is behind an H4-header (`####`). If the OPTION has a default argument, it has to be explained in the description of the OPTION. If the OPTION is also not available with a command/OPTION/content/ on the remote Podman client, the sentence about the default argument should the second to last sentence. The sentence about the default argument should be in a new line as well as the *IMPORTANT* sentence.

 All OPTIONS are to be sorted in alphabetical order.

 Tables should be used when there is a different definition for multiple arguments, and these have to be explained. This is shown with the OPTION **--test**.\
 Lists should be used when arguments are used that do not need a definition for each argument and a single description explains them. An example is shown with **[podman-commit --change](./source/markdown/podman-commit.1.md#--change--cinstruction)**


#### **--version**, **-v**

OPTIONS can be put after the command in two different ways. Either the long version with **--option** or as the short version **-o**. If there are two ways to write an OPTION they are separated by a comma. If there are two versions of one command the long version is always shown in front. If OPTION is boolean, *true/false* are not enumerated. The default boolean argument is shown in the same way normal default arguments are displayed.\
Example: The default is **false**.\
*IMPORTANT: This OPTION is not available with the remote Podman client.*

#### **--exit**

An example of a boolean OPTION that is only available in long form.

#### **--answer**, **-a**=**active** | *disable*

The **--answer** OPTION above is an example of an OPTION that accepts two possible arguments as inputs. If a default argument is selected when the OPTION is not used in the command, it is shown in **bold**. If the OPTION is used, it must include an argument afterward. It must always be ensured that the standard argument is in the first position after the OPTION. In this example, there are two different ways to execute the command. Both possible OPTIONS have to be shown with the arguments following them.\
The default value is shown as **active**.

#### **--status**=**good** | *better* | *best*

This is an example of three arguments following an OPTION. If the number of arguments is greater than three, the arguments are **not** listed after the equal sign. The arguments must be shown in a table like in **--test**=**_test_**. This form should also be used if the understanding of the content is in danger of becoming incomprehensible. An example for this is **[podman-container-prune --filters](./source/markdown/podman-container-prune.1.md#--filterfilters)**.\
The default value is shown as **good**.

#### **--test**=**test**

OPTIONS that are followed by an equal sign include an argument after the equal sign in **bold** or *italic*. If there is a default argument that is used if the OPTION is not specified in the command, the argument after the equal sign is displayed in **bold**. All arguments must be listed and explained in the text below the OPTION.

| Argument           | Description                                                                 |
| ------------------ | --------------------------------------------------------------------------- |
| **example one**    | This argument is the default argument if the OPTION is not specified.       |
| *example two*      | If one refers to a command, one should use **bold** marks.                  |
| *example three*    | Example: In combination with **podman command** highly effective.           |
| *example four*     | Example: Can be combined with **--exit**.                                   |
| *example five*     | The fifth description                                                       |

The table shows an example for a listing of arguments. The contents in the table should be aligned left. If the content in the table conflicts with this, it can be aligned to support the understanding of the content. If there is a default argument, it **must** be listed as the first entry in the table.\
The default value is shown as **example one**.

If the number of arguments is smaller than four they have to be listed behind the OPTION as seen in the OPTION **--status**.

#### **--problem**=*problem*

OPTIONS that are followed by an equal sign that is then followed by an unspecified argument, have no default argument. If this OPTION is written with an equal sign and the argument is left empty, there will be no error, but the OPTION will be ignored. The meaning of the argument is described preferably in `one` word after the equal sign in *italic* format.

## SUBCHAPTER
For chapters that are made specifically as an individual SUBCHAPTER in a man page, the previous conditions regarding formatting apply.

There are no restrictions for the use of paragraphs and tables. Within these paragraphs and tables the previous conditions regarding formatting apply.

Strings of characters or numbers can be highlighted with `backticks`. Paths of any kind **must** be highlighted.

IMPORTANT: Only characters that are **not** part of categories mentioned before can be highlighted. This includes headers. For example it is not advised to highlight an OPTION or a **command**.

SUBHEADINGS are displayed as follows:
### SUBHEADING
Text for SUBHEADINGS.

## EXAMPLES
All EXAMPLES are listed in this section. This section should be at the end of each man page. Each EXAMPLE is always in one box. The box starts and ends with the last written line, **not** with a blank line. The `$` in front of the commands indicates that it can be run as a normal user, while the commands starting with `#` can only be run as root. If there is the need for a comment in a box the comment should have `###` in front of it.

Description of the EXAMPLE
```
### Example comment
$ podman command
$ podman command -o
$ cat $HOME/Dockerfile | podman command --option
```

Description of the EXAMPLE two
```
# podman command --status=better
```
## SEE ALSO
All commands, including commands with OPTIONS, and config-files mentioned in the manpage have to be listed here. Podman commands, including commands with OPTIONS, and config-files have to be linked. If a command is mentioned several times with different OPTIONS it just have to be linked once. All other commands, including commands with OPTIONS, and config-files just have to be mentioned. If a command is mentioned several times with different OPTIONS it just has to be linked once.

Example:
**[podman(1)](podman.1.md)**, **[podman-run(1)](podman-run.1.md)**, **[podman-create(1)](podman-create.1.md)**

## HISTORY
Normally, the dates of changes, the content of the changes and the person who provided them is listed here. Most manpages don't keep this record.

Example:\
December 2021, Originally compiled by Alexander Richter <example@redhat.com>

`Every manpage should end with an empty line.`

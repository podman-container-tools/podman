#### **--filter**=*filters*

Provide filter values.

If there is more than one filter, the `--filter` option should be passed multiple times: **--filter** *label=test* **--filter** *until=10m*.

Supported filters:

| Filter      | Description                                                                                                |
|:-----------:|------------------------------------------------------------------------------------------------------------|
| all         | [Bool] When true, remove all unused volumes (same as **--all**). When false or unset, only anonymous volumes are considered. |
| anonymous   | [Bool] Only remove volumes that are anonymous (true) or named (false).                                     |
| label       | [String] Only remove volumes, with (or without, in the case of label!=[...] is used) the specified labels. |
| label!      | [String] Only remove volumes without the specified labels.                                                  |
| until       | [DateTime] Only remove volumes created before given timestamp.                                             |
| after/since | [Volume] Filter by volumes created after the given VOLUME (name or tag)                                    |

The `label` *filter* accepts two formats. One is the `label`=*key* or `label`=*key*=*value*, which removes volumes with the specified labels. The other format is the `label!`=*key* or `label!`=*key*=*value*, which removes volumes without the specified labels.

The `until` *filter* can be Unix timestamps, date formatted timestamps, or Go duration strings (e.g. 10m, 1h30m) computed relative to the machine's time.

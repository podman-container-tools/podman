#### **--filter**=*filters*

Provide filter values.

If there is more than one filter, the `--filter` option should be passed multiple times: **--filter** *label=test* **--filter** *until=10m*.

Supported filters:

| Filter | Description                                                                           |
|:------:|---------------------------------------------------------------------------------------|
| label  | Only remove networks with (or without, in case label!=[...] is used) specified labels |
| until  | Only remove networks created before given timestamp                                   |

The `label` *filter* accepts two formats. One is the `label`=*key* or `label`=*key*=*value*, which removes networks with the specified labels. The other format is the `label!`=*key* or `label!`=*key*=*value*, which removes networks without the specified labels.

The `until` *filter* can be Unix timestamps, date formatted timestamps, or Go duration strings (e.g. 10m, 1h30m) computed relative to the machine's time.

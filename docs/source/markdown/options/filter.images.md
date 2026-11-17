####> This option file is used in:
####>   podman images
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--filter**, **-f**=*filter*

Provide filter values.

If there is more than one filter, the `--filter` option should be passed multiple times: **--filter** *label=test* **--filter** *until=10m*.

Supported filters:

| Filter       | Description                                                                                   |
|:------------:|-----------------------------------------------------------------------------------------------|
| id           | Filter by image ID.                                                                           |
| before       | Filter by images created before the given IMAGE (name or tag).                                |
| containers   | Filter by images with a running container.                                                    |
| dangling     | Filter by dangling (unused) images.                                                           |
| digest       | Filter by digest.                                                                             |
| intermediate | Filter by images that are dangling and have no children                                       |
| label        | Filter by images with (or without, in the case of label!=[...] is used) the specified labels. |
| manifest     | Filter by images that are manifest lists.                                                     |
| readonly     | Filter by read-only or read/write images.                                                     |
| reference    | Filter by image name.                                                                         |
| after/since  | Filter by images created after the given IMAGE (name or tag).                                 |
| until        | Filter by images created until the given duration or time.                                    |

The `id` *filter* accepts the image ID string.

The `before` *filter* accepts formats: `<image-name>[:<tag>]`, `<image id>` or `<image@digest>`.

The `containers` *filter* shows images that have a running container based on that image.

The `dangling` *filter* shows images that are taking up disk space and serve no purpose. Dangling image is a file system layer that was used in a previous build of an image and is no longer referenced by any image. They are denoted with the `<none>` tag, consume disk space and serve no active purpose.

The `digest` *filter* accepts the image digest string.

The `intermediate` *filter* shows images that are dangling and have no children.

The `label` *filter* accepts two formats. One is the `label`=*key* or `label`=*key*=*value*, which shows images with the specified labels. The other format is the `label!`=*key* or `label!`=*key*=*value*, which shows images without the specified labels.

The `readonly` *filter* accepts `true` or `false`.

The `reference` *filter* accepts regex expressions like `image:.*-alpine`.

The `after/since` *filter* accepts formats: `<image-name>[:<tag>]`, `<image id>` or `<image@digest>`.

The `until` *filter* shows images created before the given date/time. The `<timestamp>` can be Unix timestamps, date formatted timestamps (e.g. `2020-12-31`, `2021-01-01T10:00:00`), or Go duration strings (e.g. `10m`, `1h30m`).

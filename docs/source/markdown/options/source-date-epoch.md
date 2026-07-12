####> This option file is used in:
####>   podman build, farm build
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--source-date-epoch**=*seconds*

Set the "created" timestamp for the built image to this number of seconds since
the epoch (Unix time 0, i.e., 00:00:00 UTC on 1 January 1970) (default is to
use the value set in the `SOURCE_DATE_EPOCH` environment variable, or the
current time if it is not set).

The "created" timestamp is written into the image's configuration and manifest
when the image is committed, so running the same build two different times
will ordinarily produce images with different sha256 hashes, even if no other
changes were made to the Containerfile and build context.

When this flag is set, a `SOURCE_DATE_EPOCH` build arg will provide its value
for a stage in which it is declared.

When this flag is set, the image configuration's "created" timestamp is always
set to the time specified, which should allow for identical images to be built
at different times using the same set of inputs.

When this flag is set, output written as specified to the **--output** flag
will bear exactly the specified timestamp.

Conflicts with the similar **--timestamp** flag, which also sets its specified
time on the contents of new layers.

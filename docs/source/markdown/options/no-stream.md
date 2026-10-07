####> This option file is used in:
####>   podman pod stats, stats
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--no-stream**

Disable streaming <<|pod >>stats and only pull the first result, default setting is false.

Note: When `--no-stream` is enabled, because only a single sample is captured without a preceding delta interval, the reported `CPU %` reflects the cumulative average CPU utilization over the container's lifetime since it started (or `--` when not available).

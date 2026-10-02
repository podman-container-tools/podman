####> This option file is used in:
####>   podman pod stats, stats
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--no-stream**

Disable streaming <<|pod >>stats and only pull the first result, default setting is false.

Note: With **--no-stream**, the CPU percentage represents the average usage
since the container started. In streaming mode, it reflects instantaneous
usage between refresh intervals. For accurate current CPU usage, use
streaming mode or the REST API with **stream=true**.

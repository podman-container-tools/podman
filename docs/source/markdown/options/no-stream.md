####> This option file is used in:
####>   podman pod stats, stats
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--no-stream**

Disable streaming <<|pod >>stats and only pull the first result, default setting is false.

Note: With **--no-stream**, the CPU percentage is a coarse snapshot measured
over a brief sampling window (~100 ms). In streaming mode, it reflects
usage between 5-second refresh intervals for a smoother reading. For
sustained CPU monitoring, use streaming mode or the REST API with
**stream=true**.

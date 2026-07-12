####> This option file is used in:
####>   podman create, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--oom-score-adj**=*num*

Tune the host's OOM preferences for containers (accepts values from **-1000** to **1000**).

When running in rootless mode, the specified value can't be lower than
the oom_score_adj for the current process. In this case, the
oom-score-adj is clamped to the current process value.

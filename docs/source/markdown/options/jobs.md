####> This option file is used in:
####>   podman build, farm build
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--jobs**=*number*

Run up to N concurrent stages in parallel.  If the number of jobs is greater
than 1, stdin is read from /dev/null.  If 0 is specified, then there is
no limit in the number of jobs that run in parallel.

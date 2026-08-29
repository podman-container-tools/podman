

# This script is intended to be piped into by automation, in order to
# mark output lines with timing information.  For example:
#      /path/to/command |& awk --file timestamp.awk

BEGIN {
    STARTTIME=systime()
    printf "[%s] START", strftime("%T")
    printf " - All [+xxxx] lines that follow are relative to %s.\n", strftime("%FT%TZ", systime(), 1)
}

{
    printf "[%+05ds] %s\n", systime()-STARTTIME, $0
}

END {
    printf "[%s] END", strftime("%T")
    printf " - [%+05ds] total duration since %s\n", systime()-STARTTIME, strftime("%FT%TZ", systime(), 1)
}

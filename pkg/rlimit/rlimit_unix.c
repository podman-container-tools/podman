//go:build (linux || freebsd) && cgo

#include <errno.h>
#include "rlimit.h"

static struct rlimit original_nofile;
static int original_nofile_error;
static int original_nofile_saved;

static void
save_original_nofile(void)
{
  if (original_nofile_saved)
    return;

  original_nofile_saved = 1;
  if (getrlimit (RLIMIT_NOFILE, &original_nofile) < 0)
    original_nofile_error = errno;
}

/* Capture the inherited limit before Go raises it.  */
static void __attribute__((constructor)) init(void)
{
  save_original_nofile ();
}

int
podman_get_original_nofile(struct rlimit *limit)
{
  save_original_nofile ();
  *limit = original_nofile;
  return original_nofile_error;
}

int
podman_restore_original_nofile(void)
{
  save_original_nofile ();
  if (original_nofile_error)
    {
      errno = original_nofile_error;
      return -1;
    }
  return setrlimit (RLIMIT_NOFILE, &original_nofile);
}

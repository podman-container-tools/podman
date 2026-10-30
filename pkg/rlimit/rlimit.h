#ifndef PODMAN_RLIMIT_H
#define PODMAN_RLIMIT_H

#include <sys/resource.h>

int podman_get_original_nofile(struct rlimit *limit);
int podman_restore_original_nofile(void);

#endif

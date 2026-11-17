// Wrappers around libseccomp functionality that may be missing from the
// run-time libseccomp library. The functions wrapped here are declared as weak
// references (see seccomp_compat.h), meaning they resolve to NULL when the
// run-time library is older than the version that added them, so they must not
// be called directly.

#include "seccomp_compat.h"

// The API level operations were added in libseccomp v2.4.0.

unsigned int compat_api_get(void)
{
	// Return the "reserved" value of 0 to tell the caller that proper API
	// level support is not available in libseccomp.
	if (seccomp_api_get == NULL)
		return 0;

	return seccomp_api_get();
}

int compat_api_set(unsigned int level)
{
	if (seccomp_api_set == NULL)
		return -EOPNOTSUPP;

	return seccomp_api_set(level);
}


// The seccomp notify API was added in libseccomp v2.5.0.

int compat_notify_alloc(struct seccomp_notif **req, struct seccomp_notif_resp **resp)
{
	if (seccomp_notify_alloc == NULL)
		return -EOPNOTSUPP;

	return seccomp_notify_alloc(req, resp);
}

int compat_notify_fd(const scmp_filter_ctx ctx)
{
	if (seccomp_notify_fd == NULL)
		return -EOPNOTSUPP;

	return seccomp_notify_fd(ctx);
}

void compat_notify_free(struct seccomp_notif *req, struct seccomp_notif_resp *resp)
{
	if (seccomp_notify_free == NULL)
		return;

	seccomp_notify_free(req, resp);
}

int compat_notify_id_valid(int fd, uint64_t id)
{
	if (seccomp_notify_id_valid == NULL)
		return -EOPNOTSUPP;

	return seccomp_notify_id_valid(fd, id);
}

int compat_notify_receive(int fd, struct seccomp_notif *req)
{
	if (seccomp_notify_receive == NULL)
		return -EOPNOTSUPP;

	return seccomp_notify_receive(fd, req);
}

int compat_notify_respond(int fd, struct seccomp_notif_resp *resp)
{
	if (seccomp_notify_respond == NULL)
		return -EOPNOTSUPP;

	return seccomp_notify_respond(fd, resp);
}


// The following functions were added in libseccomp v2.6.0.

int compat_precompute(scmp_filter_ctx ctx)
{
	if (seccomp_precompute == NULL)
		return -EOPNOTSUPP;

	return seccomp_precompute(ctx);
}

int compat_export_bpf_mem(const scmp_filter_ctx ctx, void *buf, size_t *len)
{
	if (seccomp_export_bpf_mem == NULL)
		return -EOPNOTSUPP;

	return seccomp_export_bpf_mem(ctx, buf, len);
}

int compat_transaction_start(const scmp_filter_ctx ctx)
{
	if (seccomp_transaction_start == NULL)
		return -EOPNOTSUPP;

	return seccomp_transaction_start(ctx);
}

int compat_transaction_commit(const scmp_filter_ctx ctx)
{
	if (seccomp_transaction_commit == NULL)
		return -EOPNOTSUPP;

	return seccomp_transaction_commit(ctx);
}

void compat_transaction_reject(const scmp_filter_ctx ctx)
{
	if (seccomp_transaction_reject == NULL)
		return;

	seccomp_transaction_reject(ctx);
}

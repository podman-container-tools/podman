#ifndef SECCOMP_COMPAT_H
#define SECCOMP_COMPAT_H

#include <errno.h>
#include <stdlib.h>
#include <seccomp.h>

// Minimally required version during compile time.
#if (SCMP_VER_MAJOR < 2) || \
    (SCMP_VER_MAJOR == 2 && SCMP_VER_MINOR < 3) || \
    (SCMP_VER_MAJOR == 2 && SCMP_VER_MINOR == 3 && SCMP_VER_MICRO < 1)
#error This package requires libseccomp >= v2.3.1
#endif


// Macros that are missing from older libseccomp headers. Being macros, they
// only need an #ifndef guard, so no version tracking is needed for them.

#define ARCH_BAD ~0

#ifndef SCMP_ARCH_PPC
#define SCMP_ARCH_PPC ARCH_BAD
#endif

#ifndef SCMP_ARCH_PPC64
#define SCMP_ARCH_PPC64 ARCH_BAD
#endif

#ifndef SCMP_ARCH_PPC64LE
#define SCMP_ARCH_PPC64LE ARCH_BAD
#endif

#ifndef SCMP_ARCH_S390
#define SCMP_ARCH_S390 ARCH_BAD
#endif

#ifndef SCMP_ARCH_S390X
#define SCMP_ARCH_S390X ARCH_BAD
#endif

#ifndef SCMP_ARCH_PARISC
#define SCMP_ARCH_PARISC ARCH_BAD
#endif

#ifndef SCMP_ARCH_PARISC64
#define SCMP_ARCH_PARISC64 ARCH_BAD
#endif

#ifndef SCMP_ARCH_RISCV64
#define SCMP_ARCH_RISCV64 ARCH_BAD
#endif

#ifndef SCMP_ARCH_LOONGARCH64
#define SCMP_ARCH_LOONGARCH64 ARCH_BAD
#endif

#ifndef SCMP_ARCH_M68K
#define SCMP_ARCH_M68K ARCH_BAD
#endif

#ifndef SCMP_ARCH_SH
#define SCMP_ARCH_SH ARCH_BAD
#endif

#ifndef SCMP_ARCH_SHEB
#define SCMP_ARCH_SHEB ARCH_BAD
#endif

#ifndef SCMP_ACT_LOG
#define SCMP_ACT_LOG 0x7ffc0000U
#endif

#ifndef SCMP_ACT_KILL_PROCESS
#define SCMP_ACT_KILL_PROCESS 0x80000000U
#endif

#ifndef SCMP_ACT_KILL_THREAD
#define SCMP_ACT_KILL_THREAD	0x00000000U
#endif

#ifndef SCMP_ACT_NOTIFY
#define SCMP_ACT_NOTIFY 0x7fc00000U
#endif


// Definitions that are only available in newer libseccomp headers. Unlike
// functions (see below), these can not be resolved at run time, so they have
// to be provided here when compiling against older headers.
//
// The SCMP_FLTATR_* ones are enum members rather than macros, so, unlike the
// above, the preprocessor can not tell whether they are already defined, and
// an explicit version check is needed instead.

#if SCMP_VER_MAJOR == 2 && SCMP_VER_MINOR < 4

// SCMP_FLTATR_CTL_LOG was added in v2.4.0.
#define SCMP_FLTATR_CTL_LOG 6

#endif // < 2.4.0


#if SCMP_VER_MAJOR == 2 && SCMP_VER_MINOR < 5

// The following SCMP_FLTATR_*  were added in libseccomp v2.5.0.
#define SCMP_FLTATR_CTL_SSB      7
#define SCMP_FLTATR_CTL_OPTIMIZE 8
#define SCMP_FLTATR_API_SYSRAWRC 9

// The seccomp notify API structs were added in v2.5.0. Their layout is
// dictated by the kernel UAPI, so it is safe to define them here.

struct seccomp_data {
	int nr;
	__u32 arch;
	__u64 instruction_pointer;
	__u64 args[6];
};

struct seccomp_notif {
	__u64 id;
	__u32 pid;
	__u32 flags;
	struct seccomp_data data;
};

struct seccomp_notif_resp {
	__u64 id;
	__s64 val;
	__s32 error;
	__u32 flags;
};

#endif // < 2.5.0


#if SCMP_VER_MAJOR == 2 && SCMP_VER_MINOR < 6

#define SCMP_FLTATR_CTL_WAITKILL 10

#endif // < 2.6.0


// Functions that were added after v2.3.1 are declared as weak references, so
// that a binary compiled against a newer libseccomp can still be loaded and
// used with an older run-time libseccomp. Without this, the dynamic linker
// fails to resolve the newer symbols, and the program does not even start.
//
// A weak reference to a symbol that is missing at run time resolves to NULL
// rather than being an error, so every such function must only be called via
// its compat_* wrapper below, which checks for availability first.
//
// Declaring these unconditionally serves a second purpose: when compiling
// against headers older than the version that added a function, the weak
// declaration is the only declaration, replacing the dummy implementations
// that used to be needed to make the package compile.

#define SCMP_WEAK __attribute__((weak))

// Forward declarations, so that the notify prototypes below do not depend on
// which of seccomp.h, linux/seccomp.h, or the compat block above ends up
// defining these structs. Only pointers to them are used here.
struct seccomp_notif;
struct seccomp_notif_resp;

// Added in v2.4.0.
SCMP_WEAK unsigned int seccomp_api_get(void);
SCMP_WEAK int seccomp_api_set(unsigned int level);

// Added in v2.5.0.
SCMP_WEAK int seccomp_notify_alloc(struct seccomp_notif **req, struct seccomp_notif_resp **resp);
SCMP_WEAK int seccomp_notify_fd(const scmp_filter_ctx ctx);
SCMP_WEAK void seccomp_notify_free(struct seccomp_notif *req, struct seccomp_notif_resp *resp);
SCMP_WEAK int seccomp_notify_id_valid(int fd, uint64_t id);
SCMP_WEAK int seccomp_notify_receive(int fd, struct seccomp_notif *req);
SCMP_WEAK int seccomp_notify_respond(int fd, struct seccomp_notif_resp *resp);

// Added in v2.6.0.
SCMP_WEAK int seccomp_precompute(scmp_filter_ctx ctx);
SCMP_WEAK int seccomp_export_bpf_mem(const scmp_filter_ctx ctx, void *buf, size_t *len);
SCMP_WEAK int seccomp_transaction_start(const scmp_filter_ctx ctx);
SCMP_WEAK int seccomp_transaction_commit(const scmp_filter_ctx ctx);
SCMP_WEAK void seccomp_transaction_reject(const scmp_filter_ctx ctx);


// Wrappers around the weakly referenced functions above; implemented in
// seccomp_compat.c. These are what the Go code calls. Normally, a missing
// function is already ruled out by the version and API level checks done in
// Go, and these wrappers are merely a safety net making sure a NULL pointer
// is never called.

unsigned int compat_api_get(void);
int compat_api_set(unsigned int level);

int compat_notify_alloc(struct seccomp_notif **req, struct seccomp_notif_resp **resp);
int compat_notify_fd(const scmp_filter_ctx ctx);
void compat_notify_free(struct seccomp_notif *req, struct seccomp_notif_resp *resp);
int compat_notify_id_valid(int fd, uint64_t id);
int compat_notify_receive(int fd, struct seccomp_notif *req);
int compat_notify_respond(int fd, struct seccomp_notif_resp *resp);

int compat_precompute(scmp_filter_ctx ctx);
int compat_export_bpf_mem(const scmp_filter_ctx ctx, void *buf, size_t *len);
int compat_transaction_start(const scmp_filter_ctx ctx);
int compat_transaction_commit(const scmp_filter_ctx ctx);
void compat_transaction_reject(const scmp_filter_ctx ctx);


// The highest API level that can possibly be supported by the compile-time
// libseccomp headers, regardless of what the run-time library reports. This
// mirrors the version gates in the Go code, which use the lower of the
// compile-time and the run-time version: functionality added after the
// compile-time version can never be used, even if a newer library is loaded
// at run time.
//
// The API level to version mapping is documented in the seccomp_api_get(3)
// man page.
#if SCMP_VER_MAJOR == 2 && SCMP_VER_MINOR < 4
#define SCMP_COMPAT_MAX_API_LEVEL 2
#elif SCMP_VER_MAJOR == 2 && SCMP_VER_MINOR == 4
#define SCMP_COMPAT_MAX_API_LEVEL 3
#elif SCMP_VER_MAJOR == 2 && SCMP_VER_MINOR == 5
#define SCMP_COMPAT_MAX_API_LEVEL 6
#else
// TODO: bump this (and add a branch above) whenever a new libseccomp
// version introduces a new API level. Until then, a newer libseccomp is
// capped to the highest level known here, which is the safe direction to
// err in, but does mean its new functionality stays unavailable.
#define SCMP_COMPAT_MAX_API_LEVEL 7
#endif

#endif // SECCOMP_COMPAT_H

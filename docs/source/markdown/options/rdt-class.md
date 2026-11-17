####> This option file is used in:
####>   podman create, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--rdt-class**=*intel-rdt-class-of-service*

Rdt-class sets the class of service (CLOS or COS) for the container to run in. Based on the Cache Allocation Technology (CAT) feature that is part of Intel's Resource Director Technology (RDT) feature set, all container processes will run within the pre-configured COS, representing a part of the cache. The COS has to be created and configured using a pseudo file system (usually mounted at `/sys/fs/resctrl`) that the resctrl kernel driver provides. Assigning the container to a COS requires root privileges and thus doesn't work in a rootless environment. Currently, the feature is only supported using `runc` as a runtime. See <https://docs.kernel.org/arch/x86/resctrl.html> for more details on creating a COS before a container can be assigned to it.

####> This option file is used in:
####>   podman machine init, machine set
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--user-mode-networking**

This option can only be used for the WSL provider on Windows. On all other
platforms this option is ignored and user mode networking will always be
`true` there because these providers always depend on gvproxy (our user
mode networking tool for the VMs)

In contrast, The Windows/WSL backend defaults to `false`, and follows the
standard WSL network setup.
Changing this setting to `true` on Windows/WSL informs Podman to replace
the WSL networking setup on start of this machine instance with a user-mode
networking distribution. Since WSL shares the same kernel across
distributions, all other running distributions reuses this network.
Likewise, when the last machine instance with a `true` setting stops, the
original networking setup is restored.

In some VPN configurations the VPN may drop traffic from alternate network
interfaces, including VM network devices. By enabling user-mode networking
VPNs observe all podman machine traffic as coming from the host, bypassing
the problem.

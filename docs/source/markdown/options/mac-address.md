####> This option file is used in:
####>   podman create, pod create, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--mac-address**=*address*

<<Container|Pod>> network interface MAC address (e.g. 92:d0:c6:0a:29:33)
This option can only be used if the <<container|pod>> is joined to only a single network - i.e., **--network=_network-name_** is used at most once -
and if the <<container|pod>> is not joining another container's network namespace via **--network=container:_id_**.

Remember that the MAC address in an Ethernet network must be unique.
The IPv6 link-local address is based on the device's MAC address
according to RFC4862.

To specify multiple static MAC addresses per <<container|pod>>, set multiple networks using the **--network** option with a static MAC address specified for each using the `mac` mode for that option.

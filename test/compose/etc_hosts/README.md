etc hosts
===========

This test mounts a /etc/hosts file in the host containing an entry `foobar`, then creates a container with an alias of the same hostname.

Validation
------------

* No /etc/hosts entries are copied from the host. There should be only one entry of the hostname, which is the IP address of the alias.
* The hostname is resolved to the IP address of the alias.

cdi devices
===========

This test copies a CDI device file on a tmpfs mounted on /etc/cdi, then checks that the CDI device in the compose file is present in a container.  The test is skipped when running as rootless.

Validation
------------

* The CDI device is present in the container.

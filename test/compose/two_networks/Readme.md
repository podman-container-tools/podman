two networks
===============

This test checks that we can create containers with more than one network.

Validation
------------
* podman container inspect two_networks_con1_1 --format '{{len .NetworkSettings.Networks}}' shows 2

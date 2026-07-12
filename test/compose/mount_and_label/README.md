mount and label
===============

This test creates a container with a mount (not volume) and also adds a label to the container.

Validation
------------
* curl http://localhost:5000 and verify message
* inspect the container to make the label exists on it

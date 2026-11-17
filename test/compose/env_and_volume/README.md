environment variable and volume
===============

This test creates two containers both of which are running flask.  The first container has
an environment variable called PODMAN_MSG.  That container pipes the contents of PODMAN_MSG
to a file on a shared volume between the containers.  The second container then reads the
file are returns the PODMAN_MSG value via flask (http).

Validation
------------
* curl http://localhost:5000 and verify message
* curl http://localhost:5001 and verify message

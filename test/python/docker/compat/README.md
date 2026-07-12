# Docker regression test

Python test suite to validate Podman endpoints using docker library (aka docker-py).
See [Docker SDK for Python](https://docker-py.readthedocs.io/en/stable/index.html).

## Running Tests

To run the tests locally in your sandbox (Fedora 32,33):

```shell
# dnf install python3-docker
```

### Run the entire test suite

All commands are run from the root of the repository.

```shell
# python3 -m unittest discover -s test/python/docker
```

Passing the -v option to your test script will instruct unittest.main() to enable a higher level of verbosity, and produce detailed output:

```shell
# python3 -m unittest -v discover -s test/python/docker
```

### Run a specific test class

```shell
# python3 -m unittest -v test.python.docker.compat.test_images.TestImages
```

### Run a specific test within the test class

```shell
# python3 -m unittest test.python.docker.compat.test_images.TestImages.test_tag_valid_image
```

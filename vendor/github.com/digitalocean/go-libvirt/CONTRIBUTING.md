Contributing
============

The `go-libvirt` project makes use of the [GitHub Flow](https://guides.github.com/introduction/flow/)
for contributions.

If you'd like to contribute to the project, please
[open an issue](https://github.com/digitalocean/go-libvirt/issues/new) or find an
[existing issue](https://github.com/digitalocean/go-libvirt/issues) that you'd like
to take on.  This ensures that efforts are not duplicated, and that a new feature
aligns with the focus of the rest of the repository.

Once your suggestion has been submitted and discussed, please be sure that your
code meets the following criteria:
  - code is completely `gofmt`'d
  - new features or codepaths have appropriate test coverage
  - `go test ./...` passes
  - `go vet ./...` passes
  - `golint ./...` returns no warnings, including documentation comment warnings

In addition, if this is your first time contributing to the `go-libvirt` project,
add your name and email address to the
[AUTHORS](https://github.com/digitalocean/go-libvirt/blob/master/AUTHORS) file
under the "Contributors" section using the format:
`First Last <email@example.com>`.

Finally, submit a pull request for review!

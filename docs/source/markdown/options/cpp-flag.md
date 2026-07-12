####> This option file is used in:
####>   podman build, farm build
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--cpp-flag**=*flags*

Set additional flags to pass to the C Preprocessor cpp(1). Containerfiles ending with a ".in" suffix is preprocessed via cpp(1). This option can be used to pass additional flags to cpp.Note: You can also set default CPPFLAGS by setting the BUILDAH_CPPFLAGS environment variable (e.g., export BUILDAH_CPPFLAGS="-DDEBUG").

# DOCKER_HOST initialization

if ($?DOCKER_HOST) exit
if ( "$euid" == 0 ) then
        setenv DOCKER_HOST unix:///run/podman/podman.sock
else
    if ($?XDG_RUNTIME_DIR) then
        setenv DOCKER_HOST unix://$XDG_RUNTIME_DIR/podman/podman.sock
    endif
endif

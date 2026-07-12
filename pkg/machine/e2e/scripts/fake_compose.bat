@ECHO off

IF "%1"=="env" (
    goto procenv
)

echo arguments: %*
exit

:procenv

if NOT "%DOCKER_HOST%" == "" (
    echo %DOCKER_HOST%
)

if NOT "%DOCKER_BUILDKIT%" == "" (
    echo %DOCKER_BUILDKIT%
)

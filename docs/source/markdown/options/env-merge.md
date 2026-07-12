####> This option file is used in:
####>   podman create, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--env-merge**=*env*

Preprocess default environment variables for the containers. For example
if image contains environment variable `hello=world` user can preprocess
it using `--env-merge hello=${hello}-some` so new value is `hello=world-some`.

Please note that if the environment variable `hello` is not present in the image,
then it'll be replaced by an empty string and so using `--env-merge hello=${hello}-some`
would result in the new value of `hello=-some`, notice the leading `-` delimiter.

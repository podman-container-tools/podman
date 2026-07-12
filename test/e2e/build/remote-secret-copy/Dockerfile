FROM alpine
COPY . .
RUN test -e hello
RUN --mount=type=secret,id=mysecret cat /run/secrets/mysecret

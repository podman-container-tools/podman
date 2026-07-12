FROM alpine
COPY . .
RUN test ! -e ignored-by-dockerignore.txt
RUN --mount=type=secret,id=MY_SECRET cat /run/secrets/MY_SECRET

FROM alpine
RUN mkdir /tmp/destination
RUN ln -s /tmp/destination /tmp/link
WORKDIR /tmp/link
CMD ["echo", "hello"]

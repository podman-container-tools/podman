FROM alpine
WORKDIR /app
RUN apk update && apk add py3-flask
COPY . /app
ENTRYPOINT ["python3"]
CMD ["app.py"]

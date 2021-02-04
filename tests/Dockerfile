FROM golang
RUN mkdir -p /app
COPY / /app
WORKDIR /app
# RUN go build ./...
# ENTRYPOINT ["tail", "-f", "/dev/null"]
ENTRYPOINT [ "go", "test", "-tags=integration", "./tests/integration/..."]

FROM golang:1.14.2-buster AS builder
ADD . /app
WORKDIR /app
ENV GO111MODULE=on
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags "-X main.sha1ver=$(git rev-parse HEAD) -X main.buildTime=$(date +'%Y-%m-%d_%T')" \
    -a -o /main .

# final stage
FROM alpine:latest
ARG SNAPSHOT_TAG="local"
ENV BUILD_HASH=$SNAPSHOT_TAG
RUN apk --no-cache add ca-certificates
COPY --from=builder /main ./
RUN chmod +x ./main
ENTRYPOINT ["./main"]
EXPOSE 8000
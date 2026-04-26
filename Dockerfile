FROM golang:1.26-alpine AS builder

RUN apk --no-cache add git

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags "-X main.sha1ver=$(git rev-parse --short HEAD) -X main.buildTime=$(date +'%Y%m%d%H%M%S')" \
    -trimpath -a -o /main .

# final stage
FROM alpine:3.22
ARG SNAPSHOT_TAG="local"
ENV BUILD_HASH=$SNAPSHOT_TAG
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /main ./main
ENTRYPOINT ["/app/main"]
EXPOSE 8000

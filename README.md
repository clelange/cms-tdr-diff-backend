# cms-tdr-diff-backend

## Build

```shell
go build -ldflags "-X main.sha1ver=$(git rev-parse HEAD) -X main.buildTime=$(date +'%Y%m%d%H%M%S')"
```

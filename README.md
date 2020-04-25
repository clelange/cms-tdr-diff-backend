# cms-tdr-diff-backend

## Build

```shell
go build -ldflags "-X main.sha1ver=$(git rev-parse HEAD) -X main.buildTime=$(date +'%Y%m%d%H%M%S')"
```

## Update image stream in OpenShift

```shell
oc tag docker.io/clelange/tdr-diff-backend-go:202004251724197e41f5 tdr-diff-backend-go:latest
```

See [Managing imagestreams](https://docs.openshift.com/container-platform/4.3/openshift_images/image-streams-manage.html#images-imagestreams-update-tag_image-streams-managing).

To list them all:

```shell
oc get is
```

Information on a specific one:

```shell
oc describe is/tdr-diff-backend-go
```

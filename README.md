# troubleshootd

Provides an API that can be used to troubleshoot issues with a host's connectivity. Tests RHP2, RHP3, and RHP4 SiaMux + QUIC.

### Default Ports
+ `8080` - API

### CLI Flags

```
-api.address string
  Explored API address (default "https://api.siascan.com")
-api.password string
  Explored API password
-http.addr string
  HTTP address to listen on (default ":8080")
-log.level value
  Log level (debug, info, warn, error) (default info)
```

# Building

```sh
go generate ./...
go build -o bin/ -tags='netgo timetzdata' -trimpath -a -ldflags '-s -w'  ./cmd/troubleshootd
```

# Docker

`troubleshootd` includes a `Dockerfile` which can be used for building and running
vaultd within a docker container. The image can also be pulled from `ghcr.io/siafoundation/troubleshootd:latest`.

## Creating the container

Create a new file named `docker-compose.yml`. You can use the following as a template.

```yml
services:
  vaultd:
    image: ghcr.io/siafoundation/troubleshootd:latest
    ports:
      - 8080:8080/tcp
    restart: unless-stopped
```

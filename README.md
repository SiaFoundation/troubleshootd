# troubleshootd

Provides an API that can be used to troubleshoot issues with a host's connectivity. Tests RHP2, RHP3, and RHP4 SiaMux + QUIC.

### Default Ports
+ `8080` - API

### Usage
#### /state 
+ `GET`: Get troubleshootd status

#### /troubleshoot
+ `POST`: Troubleshoot a host
+ Example JSON for /troubleshoot call
+ NOTE: RHP3 NetAddress is returned from RHP2 connection to host
```
{
    "publicKey": "ed25519:hostPublicKey",
    "rhp2NetAddress": "domain.tld:9982",
    "rhp4NetAddresses": [
        {
            "address": "domain.tld:9984",
            "protocol": "siamux"
        },
        {
            "address": "domain.tld:9984",
            "protocol": "quic"
        }
    ]
}
```

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
troubleshootd within a docker container. The image can also be pulled from `ghcr.io/siafoundation/troubleshootd:latest`.

## Creating the container

Create a new file named `docker-compose.yml`. You can use the following as a template.

```yml
services:
  troubleshootd:
    image: ghcr.io/siafoundation/troubleshootd:latest
    ports:
      - 8080:8080/tcp
    restart: unless-stopped
    extra_hosts:
      - "domain.tld:127.0.0.1" # Optional helper to resolve host for RHP3 test.
```

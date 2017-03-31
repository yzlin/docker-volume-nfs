# docker-volume-nfs
NFS v3/4 plugin for Docker

## Usage

```sh
make
docker plugin enable yzlin/nfs:0.1
```

```sh
docker volume create -d yzlin/nfs:0.1 -o src=user@host:/path/to/nfs nfsvol
docker run -it --rm -v nfsvol:/data alpine ls /data
```

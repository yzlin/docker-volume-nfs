# docker-volume-nfs
NFS v3/4 plugin for Docker

## Usage

```sh
docker plugin install yzlin/nfs

docker volume create -d yzlin/nfs:0.1 -o src=user@host:/path/to/nfs nfsvol
docker run -it --rm -v nfsvol:/data alpine ls /data
```

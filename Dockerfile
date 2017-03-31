FROM alpine:3.5

COPY docker-volume-nfs docker-volume-nfs

RUN set -ex \
    && apk add --no-cache \
        nfs-utils \
    && mkdir -p /run/docker/plugins /mnt/fs

CMD ["docker-volume-nfs"]

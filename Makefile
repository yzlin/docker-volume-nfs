PLUGIN_NAME=yzlin/nfs
PLUGIN_TAG=0.1

all: clean docker rootfs create

clean:
	@echo "### rm ./plugin"
	@rm -rf ./plugin

docker:
	@echo "### docker build: builder image"
	@docker build -t ${PLUGIN_NAME}:builder -f Dockerfile.dev .
	@echo "### extract docker-volume-nfs"
	@docker create --name tmp ${PLUGIN_NAME}:builder
	@docker cp tmp:/go/bin/docker-volume-nfs .
	@docker rm -vf tmp
	@docker rmi ${PLUGIN_NAME}:builder
	@echo "### docker build: rootfs image with docker-volume-nfs"
	@docker build -t ${PLUGIN_NAME}:rootfs .

rootfs:
	@echo "### create rootfs directory in ./plugin/rootfs"
	@mkdir -p ./plugin/rootfs
	@docker create --name tmp ${PLUGIN_NAME}:rootfs
	@docker export tmp | tar -x -C ./plugin/rootfs
	@echo "### copy config.json to ./plugin/"
	@cp config.json ./plugin/
	@docker rm -vf tmp

create:
	@echo "### remove existing plugin ${PLUGIN_NAME}:${PLUGIN_TAG} if exists"
	@docker plugin rm -f ${PLUGIN_NAME}:${PLUGIN_TAG} || true
	@echo "### create new plugin ${PLUGIN_NAME}:${PLUGIN_TAG} from ./plugin"
	@docker plugin create ${PLUGIN_NAME}:${PLUGIN_TAG} ./plugin

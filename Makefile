.PHONY: all directories build-docker push-to-gcr deploy

MKDIR_P = mkdir -p
GO_BUILD = go build

BUILD_DIR = build

gcr_location = eu.gcr.io/platform-226210
build_tag = build-$(shell date +%s)

all: directories build-docker push-to-gcr

directories: ${BUILD_DIR}

${BUILD_DIR}:
	${MKDIR_P} ${BUILD_DIR}

${BUILD_DIR}/receiver: ${BUILD_DIR}
	${GO_BUILD} -o ${BUILD_DIR}/receiver src/github.com/gentoomaniac/http-gcs-proxy/receiver/receiver.go

${BUILD_DIR}/retriever: ${BUILD_DIR}
	${GO_BUILD} -o ${BUILD_DIR}/retriever src/github.com/gentoomaniac/http-gcs-proxy/retriever/retriever.go

build-docker:
	$(eval commit_id := $(shell git rev-parse HEAD))
	$(eval tag_name := $(shell git show-ref --dereference --tags | sed -n 's#^$(commit_id)\s\+refs/tags/\(.*\)\^{}$$#\1#p' | uniq))
	$(eval unstable_version := $(if $(tag_name), $(shell grep -v -- '^[0-9]\+\.[0-9]\+\.[0-9]\+$$' <<<"$(tag_name)"), ""))
	docker build -t $(image_name):$(commit_id) . --no-cache
	docker tag $(image_name):$(commit_id) $(image_name):$(build_tag)
	docker tag $(image_name):$(commit_id) $(gcr_location)/$(image_name):$(commit_id)
	$(if $(tag_name), docker tag $(image_name):$(commit_id) $(gcr_location)/$(image_name):$(tag_name))
	docker tag $(image_name):$(commit_id) $(gcr_location)/$(image_name):latest

push-to-gcr:
	$(eval commit_id := $(shell git rev-parse HEAD))
	$(eval tag_name := $(shell git show-ref --dereference --tags | sed -n 's#^$(commit_id)\s\+refs/tags/\(.*\)\^{}$$#\1#p' | uniq))
	$(eval unstable_version := $(if $(tag_name), $(shell grep -v -- '^[0-9]\+\.[0-9]\+\.[0-9]\+$$' <<<"$(tag_name)"), ""))
	docker push $(gcr_location)/$(image_name):$(commit_id)
	$(if $(tag_name), docker push $(gcr_location)/$(image_name):$(tag_name))
	docker push $(gcr_location)/$(image_name):latest

deploy:
	kubectl delete -f k8s/deployment.yml
	kubectl apply -f k8s/deployment.yml

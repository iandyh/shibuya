registry=$(GAR_LOCATION)-docker.pkg.dev/$(GCP_PROJECT)
repository = shibuya
tag=$(tag_name)
img=$(registry)/$(repository)/$(component):$(tag)
upstream = rakutentech

ifeq ($(GITHUB_REPOSITORY_OWNER), $(upstream))
	tag=$(tag_name)
else
	tag=$(GITHUB_REPOSITORY_OWNER)-$(tag_name)
endif

.PHONY: base_image
base_image:
	docker build -f Dockerfile.base -t shibuya:base .

.PHONY: api_build
api_build:
	sh build.sh

.PHONY: api_image
api_image: base_image api_build
	docker build -t $(img) -f Dockerfile .
	docker push $(img)

.PHONY: controller_build
controller_build:
	sh build.sh controller

.PHONY: controller_image
controller_image: base_image controller_build
	docker build -t $(img) -f Dockerfile --build-arg="binary_name=shibuya-controller" .
	docker push $(img)

.PHONY: helm_charts
helm_charts:
	helm package install/shibuya

.PHONY: jmeter_agent
jmeter_agent:
	sh build.sh jmeter

.PHONY: jmeter_agent_image
jmeter_agent_image: jmeter_agent
	docker build -t $(img) -f engines/jmeter/Dockerfile .
	docker push $(img)

.PHONY: coordinator_build
coordinator_build:
	sh build.sh coordinator

.PHONY: coordinator_image
coordinator_image: base_image coordinator_build
	docker build -t $(img) -f coordinator/Dockerfile --build-arg="binary_name=shibuya-coordinator" .
	docker push $(img)

.PHONY: e2e
e2e:
	go clean --testcache && go test -v -run ^TestFullAPI$$ github.com/rakutentech/shibuya/shibuya/apitests

.PHONY: alltests
alltests:
	go clean --testcache && go test -v $$(go list ./... | grep -v apitests)


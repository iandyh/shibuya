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

.PHONY: cleancache
cleancache:
	go clean --testcache

## In order to run the apitests, you need to ensure everything is running.
## You can run make shibuya && make expose
.PHONY: apitests
apitests: cleancache
	go test -v github.com/rakutentech/shibuya/shibuya/apitests...

.PHONY: alltests
alltests: cleancache
	go test -cover -v $$(go list ./... | grep -v apitests)

## You need to ensure the db is running
## You can run make db && make expose
.PHONY: modeltests
modeltests: cleancache
	go test -cover -v github.com/rakutentech/shibuya/shibuya/model

.PHONY: coordinatortests
coordinatortests: cleancache
	go test -cover -v github.com/rakutentech/shibuya/shibuya/coordinator...

.PHONY: enginetests
enginetests: cleancache
	go test -cover -v github.com/rakutentech/shibuya/shibuya/engines...

.PHONY: httplibtests
httplibtests: cleancache
	go test -cover -v github.com/rakutentech/shibuya/shibuya/http...

########### Below are for local env build up #####
## This namespace should be the same as the one in parent makefile
shibuya-controller-ns = shibuya-executors

.PHONY: shibuya_all
shibuya_all: shibuya local_coordinator engines

.PHONY: shibuya
shibuya: base_image local_api local_controller
	helm uninstall shibuya || true
	sh gen_coordinator_ca.sh $(shibuya-controller-ns)
	helm upgrade --install shibuya install/shibuya
	sleep 3
	make -C ../ expose

.PHONY: local_coordinator
local_coordinator: base_image
	# if you need to debug the controller, please use the makefile in the ingress controller folder
	# And update the image in the config.json
	sh build.sh coordinator
	docker build -f coordinator/Dockerfile --build-arg binary_name=shibuya-coordinator -t coordinator:local .
	kind load docker-image coordinator:local --name shibuya

.PHONY: local_api
local_api:
	sh build.sh api
	docker build -f Dockerfile --build-arg env=local -t api:local .
	kind load docker-image api:local --name shibuya

.PHONY: local_controller
local_controller:
	sh build.sh controller
	docker build -f Dockerfile --build-arg binary_name=shibuya-controller --build-arg env=local -t controller:local .
	kind load docker-image controller:local --name shibuya

.PHONY: jmeter
jmeter: engines/jmeter
	sh build.sh jmeter
	docker build -t shibuya:jmeter -f engines/jmeter/Dockerfile .
	kind load docker-image shibuya:jmeter --name shibuya

.PHONY: locust
locust: engines/locust
	sh build.sh locust
	docker build -t shibuya:locust -f engines/locust/Dockerfile .
	kind load docker-image shibuya:locust --name shibuya

.PHONY: engines
engines: jmeter locust

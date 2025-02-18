all: | cluster permissions pdb db prometheus grafana local_storage shibuya_all

shibuya-controller-ns = shibuya-executors
shibuya-executor-ns = shibuya-executors

.PHONY: cluster
cluster:
	-kind create cluster --name shibuya --wait 180s
	-kubectl create namespace $(shibuya-controller-ns)
	-kubectl create namespace $(shibuya-executor-ns)
	kubectl apply -f kubernetes/metricServer.yaml
	kubectl config set-context --current --namespace=$(shibuya-controller-ns)
	touch shibuya/shibuya-gcp.json

.PHONY: clean
clean:
	kind delete cluster --name shibuya
	-killall kubectl

.PHONY: prometheus
prometheus:
	kubectl -n $(shibuya-controller-ns) replace -f kubernetes/prometheus.yaml --force

.PHONY: db
db: shibuya/db kubernetes/db.yaml
	-kubectl -n $(shibuya-controller-ns) delete configmap database
	kubectl -n $(shibuya-controller-ns) create configmap database --from-file=shibuya/db/
	kubectl -n $(shibuya-controller-ns) replace -f kubernetes/db.yaml --force

.PHONY: grafana
grafana: grafana/
	helm uninstall metrics-dashboard || true
	docker build grafana/ -t metrics-dashboard:local
	kind load docker-image metrics-dashboard:local --name shibuya
	helm upgrade --install metrics-dashboard grafana/metrics-dashboard

.PHONY: shibuya_all
shibuya_all:
	make -C shibuya shibuya_all

.PHONY: expose
expose:
	-killall kubectl
	-kubectl -n $(shibuya-controller-ns) port-forward service/shibuya-metrics-dashboard 3000:3000 > /dev/null 2>&1 &
	-kubectl -n $(shibuya-controller-ns) port-forward service/shibuya-api-local 8080:8080 > /dev/null 2>&1 &
	-kubectl -n $(shibuya-controller-ns) port-forward service/db 3306:3306 > /dev/null 2>&1 &

# TODO!
# After k8s 1.22, service account token is no longer auto generated. We need to manually create the secret
# for the service account. ref: "https://kubernetes.io/docs/reference/access-authn-authz/service-accounts-admin/#manual-secret-management-for-serviceaccounts"
# So we should fetch the token details from the manually created secret instead of the automatically created ones

# the shell script used here will rely on the secret in kubernetes/service-account-secret.yaml. So please create the secret first
# then export shibuya_sa_secret={secret name}

# Then run the script below.
.PHONY: kubeconfig
kubeconfig:
	./kubernetes/generate_kubeconfig.sh $(shibuya-controller-ns) $(shibuya_sa_secret)

.PHONY: pdb
pdb:
	-kubectl -n $(shibuya-controller-ns) delete -f kubernetes/pdb.yaml
	kubectl -n $(shibuya-controller-ns) apply -f kubernetes/pdb.yaml

.PHONY: permissions
permissions:
	kubectl -n $(shibuya-executor-ns) apply -f kubernetes/roles.yaml
	kubectl -n $(shibuya-controller-ns) apply -f kubernetes/serviceaccount.yaml
	kubectl -n $(shibuya-controller-ns) apply -f kubernetes/service-account-secret.yaml
	-kubectl -n $(shibuya-executor-ns) create rolebinding shibuya --role=shibuya --serviceaccount $(shibuya-controller-ns):shibuya
	kubectl -n $(shibuya-executor-ns) replace -f kubernetes/coordinator.yaml --force
	kubectl -n $(shibuya-executor-ns) replace -f kubernetes/scraper.yaml --force

.PHONY: permissions-gcp
permissions-gcp: node-permissions permissions

.PHONY: node-permissions
node-permissions:
	kubectl apply -f kubernetes/clusterrole.yaml
	-kubectl create clusterrolebinding shibuya --clusterrole=shibuya --serviceaccount $(shibuya-controller-ns):shibuya
	kubectl apply -f kubernetes/pdb.yaml

.PHONY: local_storage
local_storage:
	docker build -t shibuya:storage local_storage
	kind load docker-image shibuya:storage --name shibuya
	kubectl -n $(shibuya-controller-ns) replace -f kubernetes/storage.yaml --force

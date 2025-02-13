#!/bin/bash
: '
The secret_name should not be changed because it is required by the api deployment manifest.
jwt_secret should not be changed easily since this will immediately reset all user sessions
'

namespace=$1
secret_name=shibuya-jwt-secret
jwt_secret=$2

if kubectl get secret -n $namespace $secret_name &>/dev/null; then
    echo "Secret $secret_name already exists. Exit..."
    exit 0
fi

kubectl -n $namespace create secret generic $secret_name --from-literal=jwt_secret=$jwt_secret


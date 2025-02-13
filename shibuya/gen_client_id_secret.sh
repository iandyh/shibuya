#!/bin/bash
: '
This script is used for generating required client_id and client_secret for oauth2 login.
The secret name needs attention because the shibuya apiserver will require the secret. So the name must match.
For Google Oauth login, the secret name must be: google-oauth2
'

###
namespace=$1
secret_name=$2
client_id=$3
client_secret=$4

if kubectl get secret -n $namespace $secret_name &>/dev/null; then
    echo "Secret $secret_name already exists. Exit..."
    exit 0
fi

kubectl -n $namespace create secret generic $secret_name --from-literal=client_id=$client_id --from-literal=client_secret=$client_secret



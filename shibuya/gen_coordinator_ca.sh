#!/bin/bash
namespace=$1
secretName=shibuya-ca-crt

if kubectl get secret -n $namespace $secretName &>/dev/null; then
    echo "Secret $secretName already exists in namespace $namespace. Exit..."
    exit 0
fi
ca_dir=$(pwd)/ca
if [ -d "$ca_dir" ]; then
    echo "$ca_dir exists"
else
    mkdir $ca_dir
fi

CANAME=$ca_dir/shibuya-rootca
openssl genrsa -out $CANAME.key 2048
echo "Making cert and key...."
openssl req -x509 -new -nodes -key $CANAME.key -sha256 -days 1826 -out $CANAME.crt -subj '/CN=shibuya-coordinator/'
kubectl -n $namespace create secret tls shibuya-ca-crt --cert=$CANAME.crt --key=$CANAME.key

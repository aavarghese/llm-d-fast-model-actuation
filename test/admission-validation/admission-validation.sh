#!/usr/bin/env bash

# Create a role and service account to use for tests.
kubectl apply -f - <<EOF
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: allowedtester
rules:
- apiGroups:
    - ""
    resources:
    - pods
    verbs:
    - get
    - list
    - create
    - patch
- apiGroups
    - ""
    resources:
    - validatingadmissionpolicies
    verbs:
    - get
    - list
EOF
kubectl create rolebinding allowedtester --role=allowedtester --serviceaccount=$(kubectl get sa default -o jsonpath={.metadata.namespace}):allowedtester

# Create separate role and service account that cannot change the admission policies.
kubectl apply -f - <<EOF
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: forbiddentester
rules:
- apiGroups:
    - ""
    resources:
    - pods
    verbs:
    - get
    - list
    - create
    - patch
- apiGroups
    - ""
    resources:
    - validatingadmissionpolicies
    verbs:
    - get
    - list
EOF
kubectl create rolebinding forbiddentester --role=forbiddentester --serviceaccount=$(kubectl get sa default -o jsonpath={.metadata.namespace}):forbiddentester

# Submit a test pod with particular annotations.
kubectl apply -f annotated_test_pod.yaml

# Submit a validation policy.
kubectl apply -f policy_stub.yaml

# Check returned API server message.
forbidden_message="Only dualpods and populator controller can modify annotations"

if [[ $(eval "$1") == $forbidden_message ]]; then
    echo "Admission test passed"
else
    echo "Admission test failed"
fi
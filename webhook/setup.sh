#!/bin/bash
set -e

echo "=== Step 1: Generate TLS certificates ==="
# Kubernetes requires HTTPS for webhooks.
# The cert must match the Service DNS name inside the cluster:
#   <service-name>.<namespace>.svc
openssl req -x509 -newkey rsa:2048 -nodes \
  -keyout tls.key -out tls.crt \
  -days 365 \
  -subj "/CN=webhook-server.default.svc" \
  -addext "subjectAltName=DNS:webhook-server.default.svc,DNS:webhook-server.default.svc.cluster.local"

echo ""
echo "=== Step 2: Create Kubernetes Secret with the TLS certs ==="
# Store the certs in a Kubernetes Secret so the webhook pod can mount them
kubectl create secret tls webhook-tls \
  --cert=tls.crt --key=tls.key \
  --dry-run=client -o yaml | kubectl apply -f -

echo ""
echo "=== Step 3: Build and load the webhook Docker image ==="
docker build -t webhook-server .
kind load docker-image webhook-server --name learning

echo ""
echo "=== Step 4: Deploy the webhook server into the cluster ==="
kubectl apply -f deploy.yaml

echo ""
echo "=== Step 5: Wait for webhook pod to be ready ==="
kubectl wait --for=condition=ready pod -l app=webhook-server --timeout=60s

echo ""
echo "=== Step 6: Register the webhook with Kubernetes ==="
# We need to base64-encode the CA cert so Kubernetes trusts our self-signed cert
CA_BUNDLE=$(cat tls.crt | base64 | tr -d '\n')
sed "s|CA_BUNDLE_PLACEHOLDER|${CA_BUNDLE}|g" webhook-config.yaml | kubectl apply -f -

echo ""
echo "=== Done! The webhook is now active. ==="
echo "Try creating a pod WITHOUT a 'team' label — it should be rejected."

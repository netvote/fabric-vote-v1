kubectl delete -f netvote-api-rc.yaml
kubectl delete -f netvote-api-svc.yaml
kubectl create -f netvote-api-rc.yaml
kubectl create -f netvote-api-svc.yaml
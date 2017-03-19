docker build -t netvote/api .
docker tag netvote/api:latest gcr.io/netvote-160820/netvote/api
gcloud docker -- push gcr.io/netvote-160820/netvote/api
#!/bin/zsh

PROJECT_ID=tony-sandbox-308422
SA_NAME=sa-workload-px-tnt-prov
ROLES=("roles/iam.securityAdmin" "roles/container.admin" "roles/iam.serviceAccountTokenCreator" "roles/secretmanager.admin" "roles/iam.serviceAccountAdmin" )
K8S_NAMESPACE=default
K8S_SA=sa-workload-px-tnt-prov

gcloud iam service-accounts create $SA_NAME --project=$PROJECT_ID

for role in "${ROLES[@]}"
do
  gcloud projects add-iam-policy-binding $PROJECT_ID \
    --member "serviceAccount:$SA_NAME@$PROJECT_ID.iam.gserviceaccount.com" \
    --role "$role"
done

gcloud iam service-accounts add-iam-policy-binding "$SA_NAME@$PROJECT_ID.iam.gserviceaccount.com" \
    --role roles/iam.workloadIdentityUser \
    --member "serviceAccount:$PROJECT_ID.svc.id.goog[$K8S_NAMESPACE/$K8S_SA]"
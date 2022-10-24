#!/bin/zsh

GIT_REPO="https://github.com/tony-mw/tenant-provisioner-test"
GIT_TOKEN_PATH="/Users/$(whoami)/.github_token"
PROJECT_NAME="bootstrap"
APP_MANIFEST_PATH="github.com/MavenWave-DevOps/px-tenant-provisioner/config/samples"
APP_NAME="tenant-provisioner"

export GIT_REPO=$GIT_REPO
GIT_TOKEN=$(cat $GIT_TOKEN_PATH)
export GIT_TOKEN

argocd-autopilot repo bootstrap
argocd-autopilot project create $PROJECT_NAME
if [ -z "$APP_MANIFEST_PATH" ]
then
  argocd-autopilot app create $APP_NAME --project $PROJECT_NAME
else
  argocd-autopilot app create $APP_NAME \
  --app $APP_MANIFEST_PATH \
  --project $PROJECT_NAME \
  --type kustomize
fi
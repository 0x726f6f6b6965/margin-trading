PROJECTNAME := $(shell basename "$(PWD)")

include .env
export $(shell sed 's/=.*//' .env)

.PHONY: deploy-init
deploy-init:
	@terraform -chdir=./deploy init

.PHONY: deploy-plan
deploy-plan:
	@terraform -chdir=./deploy plan

.PHONY: deploy-apply
deploy-apply:
	@terraform -chdir=./deploy apply
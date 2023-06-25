deepcopy:
	controller-gen object  paths="./..."

crds:
	controller-gen crd  paths="./..." output:crd:artifacts:config=config/crd/bases

generate: crds deepcopy

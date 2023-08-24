#!/bin/bash

# Check if yq command is installed, otherwise install it
if ! command -v yq &> /dev/null; then
    echo "yq command not found. You need install yq first."
    exit 0
fi

# Get the current directory
current_dir=$(pwd)
echo $current_dir
# Create a temporary directory
temp_dir=$(mktemp -d)
echo $temp_dir

kustomize build config/crd > $temp_dir/crds.yaml
cd $temp_dir
awk 'BEGIN {RS="\n---\n"} {print > ("output-" NR ".yaml")}' crds.yaml
rm -f crds.yaml

# Process each file in the directory
for file in *; do
    # Parse the YAML file and extract the value of the desired fields
    group=$(yq eval '.spec.group' $file)
    plural=$(yq eval '.spec.names.plural' $file)

    # Remove leading and trailing whitespace from the field values
    group=$(echo $group | sed -e 's/^ *//' -e 's/ *$//')
    plural=$(echo $plural | sed -e 's/^ *//' -e 's/ *$//')

    # Rename the file using the extracted field values
    mv $file "$current_dir/APP-META/docker-config/pack/crds/${group}_${plural}.yaml"
done

cd $current_dir
kustomize build config/webhook > $temp_dir/webhook.yaml
cd $temp_dir
awk 'BEGIN {RS="\n---\n"} {print > ("output-" NR ".yaml")}' webhook.yaml
rm -f webhook.yaml
# Process each file in the directory
for file in *; do
    kind=$(yq eval '.kind' $file)
    kind=$(echo $kind | sed -e 's/^ *//' -e 's/ *$//')
    if [[ "$kind" == "MutatingWebhookConfiguration" ]]; then
      sed -i '' 's/mutating-webhook-configuration/aaa-kruise-mutating-webhook-configuration/g' $file
      sed -i '' 's/name: webhook-service/name: kruise-webhook-service/g' $file
      sed -i '' 's/namespace: system/namespace: kube-system/g' $file
      mv $file "$current_dir/APP-META/docker-config/pack/crds/config/mutating.yaml"
    fi

    if [[ "$kind" == "ValidatingWebhookConfiguration" ]]; then
      sed -i '' 's/name: webhook-service/name: kruise-webhook-service/g' $file
      sed -i '' 's/namespace: system/namespace: kube-system/g' $file
      mv $file "$current_dir/APP-META/docker-config/pack/crds/config/validating.yaml"
    fi
done

# Clean up the temporary directory
cd ..
rm -rf $temp_dir
cd $current_dir

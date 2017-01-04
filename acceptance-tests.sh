#!/bin/bash

set -e
set -u

cf login -a $CF_API_URL -u $CF_USERNAME -p $CF_PASSWORD -o $CF_ORGANIZATION -s $CF_SPACE

uaac target $UAA_API_URL
uaac token client get $UAA_CLIENT_ID -s $UAA_CLIENT_SECRET

set -x

space_guid=$(cf space "${CF_SPACE}" --guid)

# Test user plan

# Create service instance
cf create-service deployer-account "${USER_PLAN_NAME}" "${SERVICE_INSTANCE_NAME}"
instance_guid=$(cf service "${SERVICE_INSTANCE_NAME}" --guid)

# User exists in UAA
user_guid=$(uaac user get "${instance_guid}" -a id | awk '{ print $2 }')

# User has space developer role
space_developers=$(cf curl "/v2/spaces/${space_guid}/developers" | jq -r '.resources[] | .metadata.guid')
echo "{$space_developers}" | grep "${user_guid}"

# Delete service instance
cf delete-service -f "${SERVICE_INSTANCE_NAME}"

# User does not have space developer role
space_developers=$(cf curl "/v2/spaces/${space_guid}/developers" | jq -r '.resources[] | .metadata.guid')
if echo "${space_developers}" | grep "${user_guid}"; then
  echo "Unexpectedly found user ${instance_guid} in CF"
  exit 1
fi

# User does not exist in UAA
if uaac client get "${instance_guid}"; then
  echo "Unexpectedly found user ${instance_guid} in UAA"
  exit 1
fi

# Test client plan

# Create service instance
cf create-service deployer-account "${CLIENT_PLAN_NAME}" "${SERVICE_INSTANCE_NAME}" -c '{"redirect_uri": ["https://cloud.gov"]}'
instance_guid=$(cf service "${SERVICE_INSTANCE_NAME}" --guid)

# User exists in UAA
uaac client get "${instance_guid}"

# Delete service instance
cf delete-service -f "${SERVICE_INSTANCE_NAME}"

# User does not exist in UAA
if uaac client get "${instance_guid}"; then
  echo "Unexpectedly found user ${instance_guid} in UAA"
  exit 1
fi

####

# Ensure service instance is deleted
teardown() {
  cf delete-service -f "${SERVICE_INSTANCE_NAME}"
}
trap teardown EXIT

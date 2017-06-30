#!/bin/bash

set -e
set -u

cf login -a $CF_API_URL -u $CF_USERNAME -p $CF_PASSWORD -o $CF_ORGANIZATION -s $CF_SPACE

uaac target $UAA_API_URL
uaac token client get $UAA_CLIENT_ID -s $UAA_CLIENT_SECRET

set -x

SERVICE_KEY_NAME="${SERVICE_KEY_NAME:-creds}"
SERVICE_KEY_NAME_B="${SERVICE_KEY_NAME_B:-creds_b}"
space_guid=$(cf space "${CF_SPACE}" --guid)

# Test user plan

# Create service instance
cf create-service "${USER_SERVICE_NAME}" "${USER_PLAN_NAME}" "${SERVICE_INSTANCE_NAME}"
cf create-service-key "${SERVICE_INSTANCE_NAME}" "${SERVICE_KEY_NAME}"
cf create-service-key "${SERVICE_INSTANCE_NAME}" "${SERVICE_KEY_NAME_B}"
key=$(cf service-key "${SERVICE_INSTANCE_NAME}" "${SERVICE_KEY_NAME}" | tail -n +2)
key_b=$(cf service-key "${SERVICE_INSTANCE_NAME}" "${SERVICE_KEY_NAME_B}" | tail -n +2)
username=$(echo "${key}" | jq -r ".username")
username_b=$(echo "${key_b}" | jq -r ".username")
binding_guid=$(cf service-key "${SERVICE_INSTANCE_NAME}" "${SERVICE_KEY_NAME}" --guid)
binding_guid_b=$(cf service-key "${SERVICE_INSTANCE_NAME}" "${SERVICE_KEY_NAME_B}" --guid)

if [ "${binding_guid}" != "${username}" ]; then
  echo "Incorrect username ${username}; expected ${binding_guid}"
fi

if [ "${binding_guid_b}" != "${username_b}" ]; then
  echo "Incorrect username ${username_b}; expected ${binding_guid_b}"
fi

# User exists in UAA
user_guid=$(uaac user get "${binding_guid}" -a id | awk '{ print $2 }')
user_guid_b=$(uaac user get "${binding_guid_b}" -a id | awk '{ print $2 }')

# User has space developer role
space_developers=$(cf curl "/v2/spaces/${space_guid}/developers" | jq -r '.resources[] | .metadata.guid')
echo "{$space_developers}" | grep "${user_guid}"
echo "{$space_developers}" | grep "${user_guid_b}"

# Delete service instance
cf delete-service-key -f "${SERVICE_INSTANCE_NAME}" "${SERVICE_KEY_NAME}"
cf delete-service-key -f "${SERVICE_INSTANCE_NAME}" "${SERVICE_KEY_NAME_B}"
cf delete-service -f "${SERVICE_INSTANCE_NAME}"

# User does not have space developer role
space_developers=$(cf curl "/v2/spaces/${space_guid}/developers" | jq -r '.resources[] | .metadata.guid')
if echo "${space_developers}" | grep "${user_guid}"; then
  echo "Unexpectedly found user ${binding_guid} in CF"
  exit 1
fi

if echo "${space_developers}" | grep "${user_guid_b}"; then
  echo "Unexpectedly found user ${binding_guid_b} in CF"
  exit 1
fi

# User does not exist in UAA
if uaac client get "${binding_guid}"; then
  echo "Unexpectedly found user ${binding_guid} in UAA"
  exit 1
fi

if uaac client get "${binding_guid_b}"; then
  echo "Unexpectedly found user ${binding_guid_b} in UAA"
  exit 1
fi

# Test client plan

# Create service instance
cf create-service "${CLIENT_SERVICE_NAME}" "${CLIENT_PLAN_NAME}" "${SERVICE_INSTANCE_NAME}"
cf create-service-key "${SERVICE_INSTANCE_NAME}" "${SERVICE_KEY_NAME}" -c '{"redirect_uri": ["https://cloud.gov"]}'
cf create-service-key "${SERVICE_INSTANCE_NAME}" "${SERVICE_KEY_NAME_B}" -c '{"redirect_uri": ["https://cloud.gov"]}'
key=$(cf service-key "${SERVICE_INSTANCE_NAME}" "${SERVICE_KEY_NAME}" | tail -n +2)
key_b=$(cf service-key "${SERVICE_INSTANCE_NAME}" "${SERVICE_KEY_NAME_B}" | tail -n +2)
client_id=$(echo "${key}" | jq -r ".client_id")
client_id_b=$(echo "${key_b}" | jq -r ".client_id")
binding_guid=$(cf service-key "${SERVICE_INSTANCE_NAME}" "${SERVICE_KEY_NAME}" --guid)
binding_guid_b=$(cf service-key "${SERVICE_INSTANCE_NAME}" "${SERVICE_KEY_NAME_B}" --guid)

if [ "${binding_guid}" != "${client_id}" ]; then
  echo "Incorrect client id ${client_id}; expected ${binding_guid}"
fi

if [ "${binding_guid_b}" != "${client_id_b}" ]; then
  echo "Incorrect client id ${client_id_b}; expected ${binding_guid_b}"
fi

# User exists in UAA
uaac client get "${binding_guid}"
uaac client get "${binding_guid_b}"

# Delete service instance
cf delete-service-key -f "${SERVICE_INSTANCE_NAME}" "${SERVICE_KEY_NAME}"
cf delete-service-key -f "${SERVICE_INSTANCE_NAME}" "${SERVICE_KEY_NAME_B}"
cf delete-service -f "${SERVICE_INSTANCE_NAME}"

# User does not exist in UAA
if uaac client get "${binding_guid}"; then
  echo "Unexpectedly found user ${binding_guid} in UAA"
  exit 1
fi

if uaac client get "${binding_guid_b}"; then
  echo "Unexpectedly found user ${binding_guid_b} in UAA"
  exit 1
fi

# Ensure service instance is deleted
teardown() {
  cf delete-service-key -f "${SERVICE_INSTANCE_NAME}" "${SERVICE_KEY_NAME}" || true
  cf delete-service-key -f "${SERVICE_INSTANCE_NAME}" "${SERVICE_KEY_NAME_B}" || true
  cf delete-service -f "${SERVICE_INSTANCE_NAME}" || true
}
trap teardown EXIT

#!/bin/bash

set -u

cf login -a $CF_API_URL -u $CF_USERNAME -p $CF_PASSWORD -o $CF_ORGANIZATION -s $CF_SPACE &> /dev/null

uaac target $UAA_API_URL &> /dev/null
uaac token client get $UAA_CLIENT_ID -s $UAA_CLIENT_SECRET &> /dev/null

function test_cloud-gov-service-account-plan () {
  local plan=$1 # from config.json
  local role=$2 # in cloud foundry: space_developer | space_auditor

  local passed_count=0
  local fail_count=0
  local results_file="cloud-gov-service-account-plan_${plan}"
  rm -f $results_file

  local svc_instance_name="${plan}-acceptance-instance"
  local svc_key_name="${plan}-acceptance-key"

  # Create service 
  cf create-service "cloud-gov-service-account" "$plan" "$svc_instance_name" &> /dev/null
  cf create-service-key "$svc_instance_name" "$svc_key_name" &> /dev/null

  # Service Key GUID should be the username
  local svc_key_guid=$(cf service-key "$svc_instance_name" "$svc_key_name" --guid)
  local username=$(cf curl "/v3/service_credential_bindings/${svc_key_guid}/details" | jq -r '.credentials.username')
  if [ "$svc_key_guid" != "$username" ]; then
    echo "FAIL: Username ${username} does not macth service key guid ${svc_key_guid}." >> $results_file
    fail_count=$((fail_count+1))
  else 
    echo "PASSED: Username ${username} matches service key guid." >> $results_file
    passed_count=$((passed_count+1))
  fi
  
  # User exists in UAA
  local uaa_user=$(uaac user get "${username}" -a id | grep "id:")
  if [ -z "$uaa_user" ]; then
    echo "FAIL: Expected user ${username} to exist in UAA." >> $results_file
    fail_count=$((fail_count+1))
  else 
    echo "PASSED: User ${username} exists in UAA." >> $results_file
    passed_count=$((passed_count+1))
  fi

  # User has the role
  local space_guid=$(cf space --guid "$CF_SPACE")
  local user_guid=$(cf curl "/v3/users?usernames=${username}" | jq -r '.resources[].guid' )
  local role_count=$(cf curl "/v3/roles?space_guids=${space_guid}&user_guids=${user_guid}&types=${role}" | jq -r '.pagination.total_results')
  if [ 0 -eq $role_count ]; then
    echo "FAIL: User ${username} is not a ${role} in ${CF_ORGANIZATION}/${CF_SPACE}." >> $results_file
    fail_count=$((fail_count+1))
  else 
    echo "PASSED: User ${username} is a ${role} in ${CF_ORGANIZATION}/${CF_SPACE}." >> $results_file
    passed_count=$((passed_count+1))
  fi

  # Delete service instance
  cf delete-service-key -f "$svc_instance_name" "$svc_key_name" &> /dev/null
  cf delete-service -f "$svc_instance_name" &> /dev/null
  
  # User does not exist in CF
  local user_count=$(cf curl "/v3/users?usernames=${username}" | jq -r '.pagination.total_results' )
  if [ 0 -ne $user_count ]; then
    echo "FAIL: User ${username} should not exist in Cloud Foundry." >> $results_file
    fail_count=$((fail_count+1))
  else 
    echo "PASSED: User ${username} no longer exists in Cloud Foundry." >> $results_file
    passed_count=$((passed_count+1))
  fi 

  # User does not exist in UAA
  local uaa_user=$(uaac user get "${username}" -a id | grep "id:")
  if [ ! -z "$uaa_user" ]; then
    echo "FAIL: User ${username} should not exist in UAA." >> $results_file
    fail_count=$((fail_count+1))
  else 
    echo "PASSED: User ${username} does not exist in UAA." >> $results_file
    passed_count=$((passed_count+1))
  fi

  echo "Offering: cloud-gov-service-account. Plan: $plan. ${passed_count} passed. ${fail_count} failed."
  cat $results_file
  echo " "

  rm -f $results_file

  if [ $fail_count -gt 0 ]; then
    return 1
  fi
}


function test_cloud-gov-service-account_offering () {
  test_cloud-gov-service-account-plan "space-auditor" "space_auditor"
  local space_auditor_return_code=$?
  test_cloud-gov-service-account-plan "space-deployer" "space_developer"
  local space_developer_return_code=$?
  
  if [ $space_auditor_return_code -gt 0 ] || [ $space_developer_return_code -gt 0 ]; then
    return 1
  else 
    return 0
  fi
}

function test_oauth-client_plan () {
  local allow_public=$1

  local passed_count=0
  local fail_count=0
  local results_file="cloud-gov-identity-provider_oauth-client"
  rm -f $results_file 

  local svc_instance_name="oauth-client-test-instance"
  local svc_key_name="oauth-client-test-key"
  local redirect_uri="https://cloud.gov"

  cf create-service "cloud-gov-identity-provider" "oauth-client" "$svc_instance_name" &> /dev/null
  
  if [ "$allow_public" == true ]; then 
    cf create-service-key "$svc_instance_name" "$svc_key_name" -c '{"redirect_uri": ["'"$redirect_uri"'"], "allowpublic": true}' &> /dev/null
  else 
    cf create-service-key "$svc_instance_name" "$svc_key_name" -c '{"redirect_uri": ["'"$redirect_uri"'"]}' &> /dev/null
  fi

  # Service Key GUID should be the client id
  local svc_key_guid=$(cf service-key "$svc_instance_name" "$svc_key_name" --guid)
  local client_id=$(cf curl "/v3/service_credential_bindings/${svc_key_guid}/details" | jq -r '.credentials.client_id')
  if [ "$svc_key_guid" != "$client_id" ]; then
    echo "FAIL: Client ID ${client_id} does not macth service key guid ${svc_key_guid}." >> $results_file
    fail_count=$((fail_count+1))
  else 
    echo "PASSED: Client ID ${client_id} matches service key guid." >> $results_file
    passed_count=$((passed_count+1))
  fi

  local uaa_client_record=$(uaac client get "${client_id}")
  # client id in uaa
  local uaa_client_id=$(echo "$uaa_client_record" | grep "client_id: $client_id")
  if [ -z "$uaa_client_id" ]; then
    echo "FAIL: Expected client ${client_id} to exist in UAA." >> $results_file
    fail_count=$((fail_count+1))
  else 
    echo "PASSED: Client ${client_id} exists in UAA." >> $results_file
    passed_count=$((passed_count+1))
  fi

  # redirect URI in UAA
    local uaa_client_redirect_uri=$(echo "$uaa_client_record" | grep "redirect_uri: $redirect_uri")
  if [ -z "$uaa_client_redirect_uri" ]; then
    echo "FAIL: Expected redirect_uri ${redirect_uri} to be set in UAA." >> $results_file
    fail_count=$((fail_count+1))
  else 
    echo "PASSED: redirect_uri ${redirect_uri} set in UAA." >> $results_file
    passed_count=$((passed_count+1))
  fi

  # Allowpublic set correctly
  local uaa_allowpublic=$(echo "$uaa_client_record" | grep "allowpublic: $allow_public")
  if [ "$allow_public" = true ] && [ ! -z "$uaa_allowpublic" ]; then
    echo "PASSED: allowpublic correctly set to true in UAA." >> $results_file
    passed_count=$((passed_count+1))
  elif [ "$allow_public" = false ] && [ -z "$uaa_allowpublic" ]; then
    echo "PASSED: allowpublic correctly set to false in UAA." >> $results_file
    passed_count=$((passed_count+1))
  else 
    echo "FAIL: allowpublic incorrectly set in UAA. Expected $allow_public." >> $results_file
    fail_count=$((fail_count+1))
  fi

  # Delete service instance
  cf delete-service-key -f "$svc_instance_name" "$svc_key_name" &> /dev/null
  cf delete-service -f "$svc_instance_name" &> /dev/null

  # Client does not exist in UAA
  uaa_client_id=$(uaac client get "${client_id}" -a client_id | grep "client_id: ${client_id}")
  if [ -z "$uaa_client_id" ]; then
    echo "PASSED: Client ${client_id} removed from UAA." >> $results_file
    passed_count=$((passed_count+1))
  else 
    echo "FAIL: Client ${client_id} not removed from UAA." >> $results_file
    fail_count=$((fail_count+1))
  fi 

  echo "Offering: cloud-gov-identity-provider. Plan: oauth-client. Allow Public: $allow_public. ${passed_count} passed. ${fail_count} failed."
  cat $results_file
  echo " "

  rm -f $results_file

  if [ $fail_count -gt 0 ]; then
    return 1
  fi

}

function test_cloud-gov-identity-provider_offering () {
  test_oauth-client_plan false
  local nopublic=$?
  test_oauth-client_plan true
  local allowpublic=$?
  
  if [ $nopublic -gt 0 ] || [ $allowpublic -gt 0 ]; then
    return 1
  else 
    return 0
  fi

}
 
test_cloud-gov-service-account_offering
cloud_gov_service_account=$?
test_cloud-gov-identity-provider_offering
cloud_gov_identity_provider=$?

if [ $cloud_gov_service_account -gt 0 ] || [ $cloud_gov_identity_provider -gt 0 ]; then
  exit 1  
else 
  exit 0
fi
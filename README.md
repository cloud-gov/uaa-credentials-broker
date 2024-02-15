Cloud Foundry UAA Credentials Broker
=====================================
[![Code Climate](https://codeclimate.com/github/cloud-gov/uaa-credentials-broker/badges/gpa.svg)](https://codeclimate.com/github/cloud-gov/uaa-credentials-broker)

This service broker allows Cloud Foundry users to provision and deprovision UAA users and clients:

* UAA users managed by the broker are scoped to a given organization and space and can be used to push applications when password authentication is needed--such as when deploying from a continuous integration service. Live example: [the cloud.gov service account service](https://cloud.gov/docs/services/cloud-gov-service-account/).
* UAA clients can be used to leverage UAA authentication in tenant applications. Live example: [leveraging cloud.gov authentication](https://cloud.gov/docs/apps/leveraging-authentication/) using the [cloud.gov identity provider service](https://cloud.gov/docs/services/cloud-gov-identity-provider/).

## Usage

### UAA users

* Create service instance:

    ```bash
    $ cf create-service cloud-gov-service-account space-deployer my-service-account
    ```

* Create service key:

    ```bash
    $ cf create-service-key my-service-account my-service-key
    ```

* Retrieve credentials from service key:

    ```bash
    $ cf service-key my-service-account my-service-key
    ```

* To rotate or deprovision when user is no longer needed, delete the service key:

    ```bash
    $ cf delete-service-key my-service-account my-service-key
    ```

### UAA clients

* Create a service instance:

    ```bash
    $ cf create-service cloud-gov-identity-provider oauth-client my-uaa-client
    ```

* Create service key:

    ```bash
    $ cf create-service-key my-uaa-client my-service-key \
        -c '{"redirect_uri": ["https://my.app.cloud.gov/auth/callback"]}'
    ```

* Retrieve credentials from service key:

    ```bash
    $ cf service-key my-uaa-client my-service-key
    ```

* To rotate or deprovision when client is no longer needed, delete the service key:

    ```bash
    $ cf delete-service-key my-uaa-client my-service-key
    ```

## Deployment

* Create UAA client:

    ```bash
    $ uaac client add uaa-credentials-broker \
        --name uaa-credentials-broker \
        --authorized_grant_types client_credentials \
        --authorities scim.write,uaa.admin,cloud_controller.admin \
        --scope uaa.none
    ```

* Update Concourse pipeline:

    ```bash
    fly -t ci set-pipeline -p uaa-credentials-broker -c pipeline.yml -l credentials.yml
    ```

## Public domain

This project is in the worldwide [public domain](LICENSE.md). As stated in [CONTRIBUTING](CONTRIBUTING.md):

> This project is in the public domain within the United States, and copyright and related rights in the work worldwide are waived through the [CC0 1.0 Universal public domain dedication](https://creativecommons.org/publicdomain/zero/1.0/).
>
> All contributions to this project will be released under the CC0 dedication. By submitting a pull request, you are agreeing to comply with this waiver of copyright interest.

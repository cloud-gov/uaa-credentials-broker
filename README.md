Cloud Foundry Deployer Account Broker
=====================================
[![Code Climate](https://codeclimate.com/github/18F/deployer-account-broker/badges/gpa.svg)](https://codeclimate.com/github/18F/deployer-account-broker)

This service broker allows Cloud Foundry users to provision and deprovision deployer accounts scoped to a given organization and space. Deployer account credentials can then be used to push applications when password authentication is needed--for example, when deploying from a continuous integration service.

## Usage

* Create service instance:

    ```bash
    $ cf create-service deployer-account deployer-account my-deployer-account
    ```

* Get dashboard link from service instance:

    ```bash
    $ cf service my-deployer-account

    Service instance: my-deployer-account
    Service: deployer-account
    ...
    Dashboard: https://fugacio.us/m/k3MtzJWVZaNlnjBYJ7FUdpW2ZkDvhmQz
    ```

* Retrieve credentials from dashboard link.

* To delete the acount, delete the service instance:

    ```bash
    $ cf delete-service my-deployer-account
    ```

## Deployment

* Create UAA client:

    ```bash
    $ uaac client add deployer-account-broker \
        --name deployer-account-broker \
        --authorized_grant_types client_credentials \
        --authorities scim.write,uaa.admin,cloud_controller.admin \
        --scope uaa.none
    ```

* Update Concourse pipeline:

    ```bash
    fly -t ci set-pipeline -p deployer-account-broker -c pipeline.yml -l credentials.yml
    ```

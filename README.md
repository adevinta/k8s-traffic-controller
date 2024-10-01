# k8s-traffic-controller
An operator that enables flexible multi-cluster routing using DNS.

This operators will listen for changes on External DNS endpoints (currently doing no action) and Ingress objects.

Ingress objects will be filtered by domain (see binding-domain in the next section) and optionally by an annotation (see annotation-filter below).
After being filtered, [Endpoints](https://github.com/kubernetes-sigs/external-dns/blob/master/docs/contributing/crd-source.md) matching the hosts specified inside ingresses will be created. These endpoints will be configured with an specific route53 parameter,
to set their weight. Weight can be provided from:
 - Command line interface (using "fake" config backend and specifying a weight)
 - Via a DynamoDB table.
 - Annotations
 - Route53 healthcheck route, re-routing failing healthchecks to other clusters having the same ingress

The AWS DynamoDB table format is given as follows:

|ClusterName| CurrentWeight| DesiredWeight|
|:---| :---| :---|


Where `ClusterName` is the Name of the cluster, `CurrentWeight` is the last weight read/set by the traffic controller and `DesiredWeight` is the target Weight.
Upon changing this last attribute, traffic controller will try to update the External DNS endpoint and will write back the table entry making CurrentWeight = DesiredWeight acknowledging the change.

## Metrics Exposed

|metric name| Help text| type| purpose| 
|:---| :---| :---| :---|
|cluster_traffic_controller_ingress_weight_desired|The desired weight of the ingress|Gauge|Exposes the value obtained from the Storage Backend for Desired weight of this cluster.|
|cluster_traffic_controller_ingress_weight_current|The current weight of the cluster|Gauge|Exposes the value obtained from the Storage Backend for Current weight of this cluster.|

In normal working conditions, values exposed in the metrics come from DynamoDB and should be equal. Occasionally they may defer if scraping occurs at the very specific moment of changing the weight, fetching it from DynamoDB but still not applied by the Reconciler.

### Possible alerting
Alert if desired != current for a significant amount of time (+15min)

# How to configure the weights

## DynamoDB

Traffic controller needs to be provided with enough AWS IAM permissions to write on the configured DynamoDB table. 

Upon initialization the traffic controller will try to read the Current/Desired Weight from DynamoDB. If an entry does not exist in the table it will be created and set to initial-weight.

Writing to DynamoDB is done by using transactions that lock the table until the operation is finished. If a traffic controller tries to access the table while there is an on going transaction
there will be an exception and the operation will be skipped (Those failed operations won't be rescheduled)

## Route53 HealthCheck

This method would activate or deactivate the traffic to one particular cluster according to the healthiness of the cluster. You need to provide an endpoint in the cluster
 for this purpose see official [AWS documentation](https://docs.aws.amazon.com/Route53/latest/DeveloperGuide/dns-failover.html) for details

## Annotations

You can further configure the weight for a single Ingress by annotating it. When present, the final weight value will be `cluster_weight*annotation weight`

Use `dns.adevinta.com/traffic-weight` with values 0 - 100 to set the weight. 

Here '0' means that the traffic is disabled, whereas '100' means that all the traffic available would go to this cluster. For example, if that cluster is getting the 5% of the total traffic, this application will get this whole 5%.

This annotation can be useful for creating canary deployments, doing migrations, etc. This is an advanced usage and should be fully understood before using in production.


## Examples

In these three cases the application won't receive traffic in the cluster. We are never stopping the traffic in absolute terms, it's just routing.

| cluster-weight | healthcheck | annotation weight | expected outcome |
|:---| :---| :---| :---|
| 100 | DOWN | 100 | no traffic for app |
| 0 | UP | 100| no traffic for app |
| 100 | UP | 0 | no traffic for app |

### Weights in different configurations

If the weight is configured in both dynamoDB and Annotations, the final result will be:  

`dnsWeight = weight_in_dynamodb (as %) * weight_in_annotation`

Examples:

| cluster 1 w. | cluster 2 w. | Annotation 1 | Annotation 2 | #1 weight.     | #2 weight.     | #1 traffic % | #2 traffic % |
| :----------- | :----------- | :----------- | :----------- | :------------- | :------------- | :--- | :--- |
| 75           | 25           | 25           | 75           | 18,75          | 18,75          | 50%  | 50%  |
| 50           | 50           |	25           | 75	        | 12.5	         | 37.5           | 25%  | 75%  | 
| 10           | 90           |	50	         | 50	        | 5	             | 45             | 10%  | 90%  |
| 0	           | 100	      | 50	         | 50	        | 0	             | 50             | 0%   | 100% |
| 5	           | 95 	      | 100	         | 0	        | 5              | 0              | 100% | 0%   |
| 100.         |.100.      | N/A (empty)  | 50.       | 100            | 50             | 66%. | 33%. |

# Command line parameters

| Flag | Default Value | Wat? |
|:------|:---------------:|:----|
|metrics-addr| 8080 | Prometheus metrics endpoint port |
|cluster-name| None | Cluster name, used to lookup the right value inside the dynamodb table |
| aws-region | eu-west-1 | AWS Region for Route53 provider |
| `binding-domain` | | Domain for creating DNS entries, domains endpoints not matching this domaing will be skipped|
|backend-type | fake | Config backend to use for configuring dns weight, posible values "fake" "dynamodb"|
|annotation-filter| none | Should an annotation be given, it will be used to filter ingress objects and skip those not matching |
| `table-name` | traffic-controller | DynamoDB table read from dynamodb backend|
|initial-weight| 0 | DNS weight for this cluster, when fake backend is specified this will be the only weight used.|
|enable-leader-election | false| Enable leader election for this controller (if you run more than one instance)|
|dev-mode| false | Enables development mode (useful for testing/developing locally). This will instruct the controller to react to ingresses despite their status is not properly updated, for example, when defining External Load Balancers that require the controller to be run inside a k8s cluster in Amazon|
|annotation-prefix| dns.adevinta.com | The prefix for the `traffic-weight` annotation. The default annotation is `dns.adevinta.com/traffic-weight` |

# Testing

## Prerequisites

By default, integration tests are disabled.
To run integration tests, ensure you have the `RUN_INTEGRATION_TESTS=true` environment variable `export RUN_INTEGRATION_TESTS=true`

Run make test


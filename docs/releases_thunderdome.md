# Testing Kubo releases with Thunderdome
This document is for running Thunderdome tests by release engineers as part of releasing Kubo.

We use Thunderdome to replay ipfs.io gateway traffic in a controlled environment against two different versions of Kubo, and we record metrics and compare them to look for logic or performance regressions before releasing a new Kubo version.

For background information about how Thunderdome works, see: https://github.com/ipfs-shipyard/thunderdome

## Prerequisites

* Ensure you have access to the "IPFS Stewards" vault in 1Password, which contains the requisite AWS Console and API credentials
* Ensure you have Docker and the Docker CLI installed
* Checkout the Thunderdome repo locally (or `git pull` to ensure it's up-to-date)
* Install AWS CLI v2: https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html
* Configure the AWS CLI
  * Configure the credentials as described in the [Thunderdome documentation](https://github.com/ipfs-shipyard/thunderdome/blob/main/cmd/thunderdome/README.md#credentials), using the credentials from 1Password
* Make sure the `thunderdome` binary is up-to-date: `go build ./cmd/thunderdome`
  
## Add & run an experiment

Create a new release configuration JSON in the `experiments/` directory, based on the most recent `kubo-release` configuration, and tweak as necessary. Generally we setup the targets to run a commit against the tag of the last release, such as:

```json
	"targets": [
		{
			"name": "kubo190-4283b9",
			"description": "kubo 0.19.0-rc1",
			"build_from_git": {
				"repo": "https://github.com/ipfs/kubo.git",
				"commit":"4283b9d98f8438fc8751ccc840d8fc24eeae6f13"
			}
		},
		{
			"name": "kubo181",
			"description": "kubo 0.18.",
			"build_from_git": {
				"repo": "https://github.com/ipfs/kubo.git",
				"tag":"v0.18.1"
			}
		}
	]
```
  
Run the experiment (where `$EXPERIMENT_CONFIG_JSON` is a path to the config JSON created above):

```shell
AWS_PROFILE=thunderdome ./thunderdome deploy --verbose --duration 120 $EXPERIMENT_CONFIG_JSON
```

This will build the Docker images, upload them to ECR, and then launch the experiment in Thunderdome. Once the experiment starts, the CLI will exit and the experiment will continue to run for the duration.

## Analyze Results

Add a log entry in https://www.notion.so/ceb2047e79f2498494077a2739a6c493 and link to it from the release issue, so that experiment results are publicly visible.

The `deploy` command will output a link to the Grafana dashboard for the experiment. We don't currently have rigorous acceptance criteria, so you should look for anomalies or changes in the metrics and make sure they are tolerable and explainable. Unexplainable anomalies should be noted in the log with a screenshot, and then root caused.


## Open a PR to merge the experiment config into Thunderdome

This is important for both posterity, and so that someone else can sanity-check the test parameters.

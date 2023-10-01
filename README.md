# AWS IPv4 Costs Viewer

This tool shows the future public IPv4 costs for a variety of AWS resources (see below) across all AWS regions from an account in a user-friendly terminal UI.

## Background

As of February 2024, AWS starts charging 3.65 USD monthly for each public IPv4 address. We will get charged regardless if we provision the IP ourselves or if AWS services provision IPv4 IPs on our behalf, such as for Load balancers.

AWS also offers a similar dashboard in the AWS console, and such data is available also in the cost visibility dashboards, but we try to also provide costs and even utilization metrics for some resources, which allow AWS customers to take more informed optimization actions. And the fact that this is open-source software makes it easier to integrate into different tools.

## Features

- Fetch and display the future IPv4 costs associated with:
  - EC2 Instances
  - Elastic IPs (EIPs)
  - Load Balancers (LBs)
  - Elastic Network Interfaces (ENIs)
- Interactive terminal UI to navigate through the data.
- Shows ELB metrics such as the amount of network traffic over the last 7 days, to inform optimization actions.
- IPv4 addresses for load balancers are determined through the DNS resolution of their public FQDN.
- Data is fetched in parallel across regions and services for faster results.
- Name tags are shown wherever possible, with failover to tags created automatically by ASGs and CloudFormation stacks.

## Further improvement ideas (contributions welcome)

- Add support for more resources included in the ENI list. (e.g. ECS, APIGW, etc.)
- Add support for additional resources not included in the ENI list. (e.g. VPN endpoints, etc.)
- Properly integrate the subnets view currently available when running with --subnets.
- Add support to dump data as CSV, JSON, YAML, XLSX, and whatever other file types may make sense.
- Add some nice anonymized screenshots to the Readme file.

## Prerequisites

- Go (1.16 or later)
- AWS CLI credentials set in the shell environment, for a user/role configured with appropriate permissions.

## Installation

```bash
go get github.com/LeanerCloud/aws-ipv4-costs-viewer
```

## Usage

After installation, you can run the tool with:

```bash
aws-ipv4-costs-viewer
```

Navigate through the UI using the arrow keys. Press `ESC` to exit.

## Related Projects

Check out our other open-source [projects](https://github.com/LeanerCloud)

- [awesome-finops](https://github.com/LeanerCloud/awesome-finops) - a more up-to-date and complete fork of [jmfontaine/awesome-finops](https://github.com/jmfontaine/awesome-finops).
- [Savings Estimator](https://github.com/LeanerCloud/savings-estimator) - estimate Spot savings for ASGs.
- [AutoSpotting](https://github.com/LeanerCloud/AutoSpotting) - convert On-Demand ASGs to Spot without config changes, automated divesification, and failover to On-Demand.
- [EBS Optimizer](https://github.com/LeanerCloud/EBSOptimizer) - automatically convert EBS volumes to GP3.
- [ec2-instances-info](https://github.com/LeanerCloud/ec2-instances-info) - Golang library for specs and pricing information about AWS EC2 instances based on the data from [ec2instances.info](https://ec2instances.info).

For more advanced features of some of these tools, as well as comprehensive cost optimization services focused on AWS, visit our commercial offerings at [LeanerCloud.com](https://www.LeanerCloud.com).

We're also working on an automated RDS rightsizing tool that converts DBs to Graviton instance types and GP3 storage. If you're interested to learn more about it, reach out to us on [Slack](https://join.slack.com/t/leanercloud/shared_invite/zt-xodcoi9j-1IcxNozXx1OW0gh_N08sjg).

## Contributing

We welcome contributions! Please submit PRs or create issues for any enhancements, bug fixes, or features you'd like to add.

## License

This project is licensed under the Open Software License 3.0 (OSL-3.0).

Copyright (c) 2023 Cristian Magherusan-Stanciu, [LeanerCloud.com](https://www.LeanerCloud.com).

## Screenshots

TBD

### Credits

- Erik Norman, for the idea and early feedback.
- This project was largely written using ChatGPT GPT-4, as you can see in detail in these ChatGPT sessions: [1](https://chat.openai.com/share/e6bb0102-af38-4f7f-90fe-1e01b6f11df2) [2](https://chat.openai.com/share/bcd10ea9-c16f-4362-be46-d45af3becfc6) [3](https://chat.openai.com/share/546b270a-2953-472e-9559-a6a9fa8ec2e9) [4](https://chat.openai.com/share/0bad817d-14ab-428f-966c-83bc372fed40) [5](https://chat.openai.com/share/47af2243-4d47-4dfe-91bb-4cf2f027699b) [6](https://chat.openai.com/share/59c9c587-311e-491e-9646-38913743ff1d) [7](https://chat.openai.com/share/fa4134b4-eb0f-4528-95e7-4c9e0a7d4512).
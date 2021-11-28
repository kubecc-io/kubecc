The e2e test suite runs on OpenStack. To run the tests, your OpenStack cloud 
must have the following optional services running:
- Heat
- Swift
- EC2 API

A file in this directory named `parameters.yaml` should be created with the 
following contents:

```yaml
image: "your image name"
flavor: "your flavor"
public_net: "your public network"
dns_nameserver:  "your dns server"
```
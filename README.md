# servicemesh

## Quick Start (Older)
[Click to deploy using Cloud Formation](https://us-west-2.console.aws.amazon.com/cloudformation/home?region=us-west-2#/stacks/new?stackName=AviK8SQuickstart&templateURL=https://s3-us-west-2.amazonaws.com/aviservicemesh/kubernetes-cluster-with-new-vpc.template)

Or use S3 url of the template: https://s3-us-west-2.amazonaws.com/aviservicemesh/kubernetes-cluster-with-new-vpc.template

[Video Tutorial](https://youtu.be/k8tjLTihnzE)


## Quick Start (avi_k8s_controller)

In order to build the project do the following:

    - mkdir -R $(GOPATH)/src/github.com/avinetworks
    - cd $(GOPATH)/src/github.com/avinetworks
    - git clone https://github.com/avinetworks/servicemesh
    - make all

#!/usr/bin/env bash

bucket=avi-servicemesh

mydir=`dirname $0`
cd $mydir
mydir=`pwd`

cd ..
root=`pwd`

cmd="aws s3 sync --acl public-read src/main/templates/ s3://${bucket}/cloudformation/templates/stage/"

echo "Copying templates to S3: ${cmd}"

$cmd
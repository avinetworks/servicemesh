#!/bin/bash

# Wait till kubernetes setup is complete
while ! test -f "/etc/kubernetes/pki/apiserver-kubelet-client.crt"; do
  sleep 30
  echo "."
done

# Get local IP
masterIP=`curl http://169.254.169.254/latest/meta-data/local-ipv4`

# Read K8S api client certificates and keys
sed -E ':a;N;$!ba;s/\r{0,1}\n/\\n/g' /etc/kubernetes/pki/apiserver-kubelet-client.crt > /tmp/temp.txt
APICRT=`cat /tmp/temp.txt`
sed -E ':a;N;$!ba;s/\r{0,1}\n/\\n/g' /etc/kubernetes/pki/apiserver-kubelet-client.key > /tmp/temp.txt
APIKEY=`cat /tmp/temp.txt`
sed -E ':a;N;$!ba;s/\r{0,1}\n/\\n/g' /etc/kubernetes/pki/ca.crt > /tmp/temp.txt
CACRT=`cat /tmp/temp.txt`

# Read the template
setup=`cat /tmp/template_setup.json`

# Update the template with corresponding values
setup=${setup//XXk8sapicrtXX/$APICRT}
setup=${setup//XXk8sapikeyXX/$APIKEY}
setup=${setup//XXk8scacrtXX/$CACRT}
setup=${setup//XXvpcidXX/{{VPCID}}}
setup=${setup//XXdomainXX/{{DOMAIN}}}
setup=${setup//XXregionXX/{{REGION}}}
setup=${setup//XXsubnetidXX/{{SUBNETID}}}
setup=${setup//XXdnsvipXX/{{DNSVIP}}}
setup=${setup//XXk8smasterXX/$masterIP}


# Store setup.json file
echo $setup | python -mjson.tool > /tmp/setup.json

# Host setup.json
#(pushd /tmp/; python -m SimpleHTTPServer &) &
docker run -d -v /tmp/:/var/www:ro -p 8000:8080 --name=avi-setup trinitronx/python-simplehttpserver
echo "Hosting avi setup on port 8000"

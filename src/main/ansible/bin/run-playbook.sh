#!/usr/bin/env bash

mydir=`dirname $0`
cd $mydir
mydir=`pwd`

cd ..
root=`pwd`

ansible-galaxy install -r requirements.yml

ansible-playbook -vvv $1 #--vault-password-file=~/.ansible/vault-password
#!/usr/bin/env bash

mydir=`dirname $0`
cd $mydir
mydir=`pwd`

cd ..
root=`pwd`

git pull

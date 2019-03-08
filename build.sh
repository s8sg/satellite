#!/bin/sh

export eTAG="latest"
echo $1
if [ $1 ] ; then
  eTAG=$1
fi

Version=$(git describe --tags --dirty)
GitCommit=$(git rev-parse HEAD)


echo Building s8sg/satellite:$eTAG

docker build --build-arg VERSION=$Version --build-arg GIT_COMMIT=$GitCommit -t s8sg/satellite:$eTAG . && \
 docker create --name satellite s8sg/satellite:$eTAG && mkdir bin && \
 docker cp satellite:/root/satellite ./bin/ && \
 docker rm -f satellite

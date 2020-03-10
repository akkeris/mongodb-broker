#!/bin/sh

export GLOG_logtostderr=1 
exec ./mongodb-broker -insecure -logtostderr=1 -stderrthreshold 0 $*

#!/bin/sh

GLOG_logtostderr=1 ./mongodb-broker -insecure -logtostderr=1 -stderrthreshold 0 $*

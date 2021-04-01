#!/bin/sh

kubectl -n kubecc port-forward service/kubecc-monitor 19090:9090 &
kubectl -n kubecc port-forward service/kubecc-scheduler 19091:9090
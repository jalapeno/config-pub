#!/bin/sh

sudo brctl addbr config-pub
sudo brctl addbr xrd06-host
sudo brctl addbr xrd09-host
sudo brctl addbr xrd15-host
sudo brctl addbr xrd46-host
sudo brctl addbr frr96-host

sudo ip link set up config-pub
sudo ip link set up xrd06-host
sudo ip link set up xrd09-host
sudo ip link set up xrd15-host
sudo ip link set up xrd46-host
sudo ip link set up frr96-host

sudo ip addr add 172.20.2.1/24 dev config-pub
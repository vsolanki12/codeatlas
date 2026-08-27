package mycontroller

import (
	etcdv2 "example.com/hypershift/v2/etcd"
	kasv2 "example.com/hypershift/v2/kas"
	"example.com/hypershift/v2/olm"
)

func registerComponents() {
	etcdv2.NewComponent()
	kasv2.NewComponent()
	olm.NewComponent()
}

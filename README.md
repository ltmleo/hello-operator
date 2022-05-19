# hello_operator
Getting started with kubernetes operators


$ operator-sdk init --domain=ltmleo.com --repo=github.com/ltmleo/hello-operator

$ go get github.com/onsi/gomega@v1.8

$ go get github.com/onsi/ginkgo@v1.11.0

$ go get github.com/go-logr/logr@v0.1.0

$ operator-sdk create api --group=config --version=v1alpha1 --kind=Hello

$ make install

$ make run ENABLE_WEBHOOKS=false

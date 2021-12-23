# provider-aws-controlapi

`provider-aws-controlapi` is a crossplane aws provider based on recently release aws cloud control api. It comes
with the following features that are meant to be refactored:

- A `ProviderConfig` type that is used to store base 64 encoded AWS secret and access key.
- A `S3` resource type that serves as an example managed resource.
- A managed resource controller that reconciles `S3` objects

It is based on crossplane provider-template, however all the libraries including kubebuilder has been updated to latest version.

## Developing

Run against a Kubernetes cluster:

```console
make run
```

Build, push, and install:

```console
make all
```

Build image:

```console
make image
```

Push image:

```console
make push
```

Build binary:

```console
make build
```

# dra-driver-memory

Kubernetes Dynamic Resource Allocation (DRA) driver for memory and hugepages resources.

This repository implements a DRA driver that enables Kubernetes clusters to manage and
assign memory and hugepages resources to workloads using the DRA framework.
In a nutshell, this is a DRA-based [memory manager](https://kubernetes.io/docs/tasks/administer-cluster/memory-manager/) replacement.

## How it Works

The driver is deployed as a DaemonSet which contains two core components bundled in the same binary:

- **DRA driver**: The frontend. It is responsable discovering the memory and hugepages topology of
the node and reporting them as allocatable resources to the Kubernetes scheduler by creating
`ResourceSlice` objects. When a resource claim is allocated, the driver generates a
CDI (Container Device Interface) specification that tells the container runtime to inject environment
variables with the allocation details into the container.

- **NRI Plugin**: The backend. It integrates with the container runtime via the Node Resource Interface (NRI).
  - For containers with memory or hugepage claims, the plugin reads the environment variables injected
    via CDI and pins the container to its assigned NUMA node(s).
  - **Optional but recommended** It sets the appropriate hugepage cgroup limits based on the allocations.
    In this mode, the driver replaces the hugepages allocation management in the kubelet.

## Resources

The driver manages the following resource types, each exposed as a separate DeviceClass:

- `dra.memory` - Regular memory (4KiB pages)
- `dra.hugepages-2m` - 2MiB hugepages (`x86_64`)
- `dra.hugepages-1g` - 1GiB hugepages (`x86_64`)

All the supported resources are reported as separate pools.
Unified accounting using `memory_hugetlb_accounting` is not supported.
The code tries to be generic and support any hugepage size, but the project is currently
tested only on `x86_64`. Support for non-`x86_64` platforms is planned for future releases.

## Device Attributes

Each memory device exposes the following attributes:

| Attribute | Type | Description |
|-----------|------|-------------|
| `resource.kubernetes.io/numaNode` | int | NUMA node where the memory resides |
| `resource.kubernetes.io/pageSize` | string | Page size (e.g., `4k`, `2m`, `1g`) |
| `resource.kubernetes.io/hugeTLB` | bool | Whether this is a hugepage resource |

Compatibility attributes for other DRA drivers are also exposed:
- `dra.cpu/numaNode` - for dra-driver-cpu
- `dra.net/numaNode` - for dranet

**The attribute naming format is not final** and subjected to change.
[thread on #wg-device-management k8s slack server](https://kubernetes.slack.com/archives/C0409NGC1TK/p1764687710269999)

## Requirements

- Kubernetes 1.34.0 or later **DRA GA required**
- cgroup v2 (without `memory_hugetlb_accounting` option)
- Container runtime with CDI support
- Container runtime with NRI plugins support

The project supplies setup helpers to make sure CDI and NRI support is enabled in the runtime.
Currently only `containerd` runtime is supported.

## Getting Started

### Installation

The driver requires invasive containerd configuration, so it is recommended to create
a kind cluster dedicated to the DRA driver. Integration in existing clusters, kind or others,
is planned for future releases.

To create a cluster with the DRA memory driver runner, just run

```bash
make ci-kind-setup
```

To clean it up, just run

```bash
make ci-kind-teardown
```

### Hugepages Provisioning

If the system does not have hugepages pre-allocated, you can provision them at runtime:

```bash
./bin/setup-hugepages provision.yaml
```

Example provisioning configuration file (`provision.yaml`):

```yaml
kind: HugePageProvision
metadata:
  name: balanced-runtime
spec:
  pages:
  - size: "2M"
    count: 4096
```

You can check out the example provisioning files in `doc/provision/`

### Example Usage

1. Create a ResourceClaimTemplate requesting hugepages:

```yaml
apiVersion: resource.k8s.io/v1
kind: ResourceClaimTemplate
metadata:
  name: hugepages-2m
spec:
  spec:
    devices:
      requests:
      - name: hp2m
        exactly:
          deviceClassName: dra.hugepages-2m
          capacity:
            requests:
              size: 256Mi
```

2. Create a Pod referencing the ResourceClaim:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: pod-with-hugepages
spec:
  containers:
  - name: app
    image: your-app:latest
    resources:
      limits:
        cpu: 1
        memory: 2Gi
      claims:
      - name: hp2m
  resourceClaims:
  - name: hp2m
    resourceClaimTemplateName: hugepages-2m
```

For regular memory allocation, use `dra.memory` as the deviceClassName:

```yaml
apiVersion: resource.k8s.io/v1
kind: ResourceClaimTemplate
metadata:
  name: memory-512m
spec:
  spec:
    devices:
      requests:
      - name: mem
        exactly:
          deviceClassName: dra.memory
          capacity:
            requests:
              size: 512Mi
```

### Container images

With the caveat that running this driver requires custom node *and* containerd configuration,
prebuilt container images are available on [quay.io](https://quay.io/repository/fromani/dra-driver-memory).

## Feature Support

### Currently Supported

- Memory and hugepages allocation with NUMA awareness
- NUMA node pinning for containers with memory claims
- Hugepage cgroup limits enforcement
- Runtime hugepages provisioning
- Multiple hugepage sizes (2MiB, 1GiB on `x86_64`)

### Not Supported

- Unified memory/hugepages accounting (`memory_hugetlb_accounting`)
- Best-effort runtime allocation of hugepages
- Memory QoS settings beyond hugepage limits

## Sharing Resource Claims

This driver strictly enforces a 1-to-1 mapping between Claims and Containers.
It does not support sharing a single ResourceClaim among multiple containers or multiple pods.
Because technical limitations, the driver does not errors out correctly in all the cases on
which a claim sharing is attempted. This is a technical limitation which we aim to improve.

In other words, this driver may fail to disallow claim sharing in your workflow.
These are bugs; please file tracking issues accordingly.

The rationale to disallow sharing is that memory is intrinsically fungible and anonymous.
Unlike devices, memory pages do not have persistent physical identities that are meaningful
to the user. Named memory regions (e.g., System V shared memory, hugetlbfs) are the exception,
not the rule, and require specific and explicit allocation strategies.

Comparing to other DRA drivers, [CPUs](https://github.com/kubernetes-sigs/dra-driver-cpu) and
GPUs are discrete resources with distinct physical identities (e.g., Core IDs, PCIe Addresses).
Sharing a CPU claim maps cleanly to time-slicing access to a specific, named hardware core.
This usage pattern fits quite closely the DRA 'Device' model.

Because memory is anonymous by default, attempting to 'share' a memory claim means more splitting
an abstract budget (quota) rather than granting access to a shared physical device.
Handling quotas is, at the moment (kubernetes 1.35.0), a core kubernetes responsability rather
than a DRA driver responsability.

As initiatives like KEP-5517 (Native Resource Management) progress, the role of DRA drivers may
expand to include quota management responsibilities.

If you have a use case that requires sharing a memory claim—specifically one that is not solved
by existing shared memory volumes (e.g., emptyDir: medium=Memory, /dev/shm, or hugetlbfs mounts)—please
file a tracking issue detailing your requirements.

## Development

### Building

```bash
# Build the driver binary
make build

# Build container image
make build-image

# Run unit tests
make test-unit

# Run linting
make lint
```

### Testing

```bash
# Set up a CI cluster
make ci-kind-setup

# Run memory allocation E2E tests - requires a cluster running with the driver deployed
make test-e2e-kind-mem

# Run hugepages E2E tests - requires a cluster running with the driver deployed and pre-provisioned hugepages
make test-e2e-kind-hp

# Clean up the CI cluster
make ci-kind-teardown
```

## License

This project is licensed under the Apache License 2.0.


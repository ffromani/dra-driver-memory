Design notes
============

Sparse and unorganized design notes, ideas and constraints. This is a living document and will serve as basis for a *future* design doc.
This is not a design doc and doesn't aim to be. Unless specified otherwise, there's never order, priority or coupling among items.

## Design Decisions

These design items are pretty much settled and won't change without significant discussion or new strong evidence.

1. memory resources are anonymous. There's no way to identify a memory region but from its properties (e.g. NUMA affinity).
   Therefore, memory resources are best modeled as pooled resources vs enumerating them explicitly.
2. memory resources will be requestable by their attributes only, indirectly.
3. The driver will report multiple resources, following the current ("classic") kubernetes resource model.
   In thee "classic" kubernetes resource model, memory is split from hugepages, which are in turn split by their size.
   In `x86_64`, we have then three resources: `memory`, `hugepages-2Mi`, `hugepages-1Gi`. This representation and split is gonna be preserved.
   A. hugepages and memory will be separate pools. [Unified accounting using `memory_hugetlb_accounting`](https://docs.kernel.org/admin-guide/cgroup-v2.html)
      is not supported as requires specific kernel settings.
   B. While this is arguably an adaptation to the status quo, there is also not obviously better resource representation either.
4. the driver is composed with 3 main building blocks. A DRA frontend layer, a CDI communication mechanism, a NRI actuaction layer.
   the NRI layer must contain no logic nor policies, and must be as direct and simple as possible actuaction layer.
   the DRA frontend portion of the driver should be as logic free as possible. The allocation logic should be pushed as much as possible in the
   scheduling side. Only node-local allocation decisions which MUST be acted upon (if at all) on the node should be implemented in the DRA
   frontend of the driver.
5. The driver should implement same functionality as the [MemoryQOS KEP](https://github.com/kubernetes/enhancements/issues/2570)

## Design Trends

These design items are solidifying and gathering consensus but there is no full certainty yet. We are converging but we're not there yet.

1. The DRA driver should take over the memory QOS setting in the cgroup hierarchy over the kubelet. We need to have a single owner/writer
   A. this is a trend because is not clear yet how the interoperability is gonna look like.
2. The DRA driver will assume the cgroup layout set and traditionally used by the kubelet
   A. this is a trend because it's an adaptation to the status quo rather than a conscious design decision.
3. The driver will not set hugepages limits, like the kubelet currently does
   A. if we do so, is not fully clear what a "hugepage claim" actually represent. Logically, a claim should imply a guarantee of sorts
      but besides a implicit guarantee of NUMA alignment, there are nothing more. What would prevent a buggy workload to ask for more pages
      that it claims? This misbehavior will violate other claims to get at least the amount of hugepages they requested.

## Open points

These design items are very much work in progress and there is no clear direction yet. Most if not all the options are still on the table.

1. Should we allow a best-effort runtime allocation of hugepages?

# Shop

Hand-written text stays as it is.

<!-- specarch:generate stateDiagram Order -->
```mermaid
stateDiagram-v2
  [*] --> open
  open --> paid : payOrder
  paid --> [*]
```
<!-- specarch:end -->

More hand-written text.

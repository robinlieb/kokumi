# Local Dex OIDC

Dex runs as a sidecar in the kokumi-server pod. Issuer `http://localhost:5556` is same inside the pod and on your host.

## Users

| Email | Password | Group | SA | Access |
|---|---|---|---|---|
| `admin@kokumi.dev` | `password` | `kokumi-admin` | `kokumi-admin` | Full |
| `editor@kokumi.dev` | `password` | `kokumi-editor` | `kokumi-editor` | Write |
| `viewer@kokumi.dev` | `password` | `kokumi-viewer` | `kokumi-viewer` | Read |
| `approver1@kokumi.dev` | `password` | `kokumi-editor`, `release-approvers` | `kokumi-editor` | Write, eligible approver |
| `approver2@kokumi.dev` | `password` | `kokumi-editor`, `release-approvers` | `kokumi-editor` | Write, eligible approver |

Approvals can only be submitted by OIDC users (the built-in `admin` login is
a shared account and cannot vote). Members of `release-approvers` are eligible
for the gated sample `config/samples/delivery_v1alpha1_order_approval.yaml`.

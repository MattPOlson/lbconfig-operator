# Connection Draining Examples

This directory contains example manifests demonstrating the graceful connection draining feature for ExternalLoadBalancer resources.

## Overview

Connection draining allows pool members to be removed gracefully without dropping active connections. When enabled, the operator:

1. **Disables** the pool member (stops new connections)
2. **Waits** for a configurable timeout (allows existing connections to complete)
3. **Deletes** the pool member after the timeout expires

## Configuration

The drain feature is configured in the ExternalLoadBalancer spec:

```yaml
spec:
  drain:
    enabled: true           # Enable/disable graceful draining (default: false)
    timeoutSeconds: 30      # Timeout in seconds (default: 30, min: 0, max: 3600)
```

## Use Cases

### Short-Lived Connections (30-60 seconds)
**Recommended for:**
- REST APIs with typical request/response patterns
- Standard HTTP web applications
- API gateways

**Example:** [lb_v1_externalloadbalancer-drain-default.yaml](./lb_v1_externalloadbalancer-drain-default.yaml)

```yaml
drain:
  enabled: true
  timeoutSeconds: 30
```

### Long-Running Connections (5-10 minutes)
**Recommended for:**
- WebSocket connections
- Server-Sent Events (SSE)
- Long-polling applications
- File upload/download services
- Streaming APIs

**Example:** [lb_v1_externalloadbalancer-drain-longrunning.yaml](./lb_v1_externalloadbalancer-drain-longrunning.yaml)

```yaml
drain:
  enabled: true
  timeoutSeconds: 300  # 5 minutes
```

### Drain Disabled (Immediate Removal)
**Recommended for:**
- Very short-lived connections (< 1 second)
- Applications that handle reconnection gracefully
- When immediate member removal is required

**Example:** [lb_v1_externalloadbalancer-drain-disabled.yaml](./lb_v1_externalloadbalancer-drain-disabled.yaml)

```yaml
drain:
  enabled: false
```

## Provider Support

All load balancer providers support graceful draining:

| Provider | Implementation | Notes |
|----------|---------------|-------|
| **F5 BigIP** | Session user-disabled | Native graceful drain support |
| **Citrix ADC/NetScaler** | Graceful service disable | Enters TROFS (Transition Out of Service) state |
| **HAProxy** | Maintenance mode | Uses DataPlane API maintenance mode |
| **Dummy** | Logging only | For testing purposes |

## How It Works

### Time-Based Approach

The drain feature uses a **time-based approach** rather than tracking active connections:

- ✅ **Simple and predictable** - Works consistently across all load balancer types
- ✅ **Provider-agnostic** - No complex connection polling required
- ⚠️ **Important:** Set `timeoutSeconds` longer than your worst-case connection duration

### Example Timeline

```
Time 0s:   Node becomes NotReady
           → Pool member disabled (no new connections)
           → Existing connections continue

Time 30s:  Timeout expires
           → Pool member deleted from load balancer
```

### State Tracking

The operator tracks draining members in the ExternalLoadBalancer status:

```yaml
status:
  drainingMembers:
    - poolName: "Pool-api-6443"
      node:
        name: "worker-3"
        host: "10.0.1.23"
      port: 6443
      startTime: "2025-12-10T21:50:00Z"
```

This state persists across reconciliation loops and operator restarts.

## Choosing the Right Timeout

### Guidelines by Application Type

| Application Type | Recommended Timeout | Reason |
|-----------------|---------------------|--------|
| REST APIs | 30-60 seconds | Typical request duration |
| gRPC services | 60-120 seconds | Longer streaming RPCs |
| WebSockets | 300-600 seconds | Long-lived connections |
| File uploads | 600-3600 seconds | Large file transfers |
| Streaming video | 600-3600 seconds | Long playback sessions |

### Monitoring

Since the drain is time-based, monitor your application metrics to ensure:
- No 502/503 errors during node draining
- No connection reset errors
- Graceful handling of member removal

## Deployment Scenarios

### Rolling Cluster Upgrades

```yaml
drain:
  enabled: true
  timeoutSeconds: 120  # 2 minutes for safe transition
```

Prevents connection drops during Kubernetes cluster upgrades when nodes are cordoned and drained.

### Autoscaling Scale-Down

```yaml
drain:
  enabled: true
  timeoutSeconds: 60  # 1 minute for graceful scale-down
```

Allows pods to complete requests before nodes are scaled down.

### Label-Based Routing Changes

```yaml
drain:
  enabled: true
  timeoutSeconds: 30  # 30 seconds for label updates
```

Prevents drops when changing node labels for router sharding or workload placement.

## Testing

You can test the drain feature using the Dummy provider:

```bash
# Apply example with drain enabled
kubectl apply -f config/samples/lb_v1_externalloadbalancer-drain-default.yaml

# Watch the operator logs
kubectl logs -n lbconfig-operator-system deployment/lbconfig-operator-controller-manager -f

# Trigger drain by removing a node label
kubectl label node <node-name> node-role.kubernetes.io/master-

# Observe the 3-phase process in logs:
# 1. "Starting graceful drain for member"
# 2. "Member still draining" (if reconciled during wait)
# 3. "Drain timeout expired, deleting member"
```

## Troubleshooting

### Connections still dropped despite drain enabled

**Cause:** Timeout too short for your connection duration

**Solution:** Increase `timeoutSeconds` to exceed your worst-case connection time:

```yaml
drain:
  enabled: true
  timeoutSeconds: 600  # Increase to 10 minutes
```

### Members not being deleted

**Cause:** Check operator logs for errors

**Solution:**
```bash
kubectl logs -n lbconfig-operator-system deployment/lbconfig-operator-controller-manager
```

Look for errors in `DisablePoolMember` or `DeletePoolMember` calls.

### Drain taking longer than expected

**Cause:** Operator requeue intervals

**Solution:** This is normal. The operator rechecks draining members periodically. Total time = `timeoutSeconds` + reconciliation overhead (usually < 5 seconds).

## Additional Examples

See the `config/samples/` directory for more examples:
- `lb_v1_externalloadbalancer-master.yaml` - Master nodes (without drain)
- `lb_v1_externalloadbalancer-infra.yaml` - Infra nodes (without drain)
- `lb_v1_externalloadbalancer-haproxy.yaml` - HAProxy provider (without drain)

## References

- [GitHub Issue #492](https://github.com/carlosedp/lbconfig-operator/issues/492) - Original feature request
- [F5 BigIP Documentation](https://my.f5.com/manage/s/article/K13310) - Disable nodes for maintenance
- [Citrix ADC Graceful Shutdown](https://docs.netscaler.com/en-us/citrix-adc/current-release/load-balancing/load-balancing-advanced-settings/graceful-shutdown.html)
- [HAProxy Runtime API](https://www.haproxy.com/blog/dynamic-configuration-haproxy-runtime-api) - Dynamic configuration

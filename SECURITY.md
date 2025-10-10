# Security Policy

## Supported Versions

We release patches for security vulnerabilities. Currently supported versions:

| Version | Supported          |
| ------- | ------------------ |
| 1.x.x   | :white_check_mark: |
| < 1.0   | :x:                |

## Reporting a Vulnerability

We take security seriously. If you discover a security vulnerability, please follow these steps:

### Private Disclosure

**DO NOT** create a public GitHub issue for security vulnerabilities.

Instead, please report security issues by emailing:

**Email:** security@yourdomain.com (or create a GitHub Security Advisory)

### What to Include

Please include the following information:

1. **Description** - Detailed description of the vulnerability
2. **Impact** - What can an attacker do with this vulnerability?
3. **Reproduction** - Step-by-step instructions to reproduce
4. **Affected versions** - Which versions are affected?
5. **Suggested fix** - If you have ideas for a fix (optional)

### Example Report

```
Subject: [SECURITY] Clock manipulation vulnerability

Description:
A malicious actor with system access can manipulate the system clock
to cause ID collisions in snowflake generation.

Impact:
- Duplicate IDs may be generated
- Data integrity issues in distributed systems

Reproduction:
1. Start generator with worker ID 1
2. Set system clock back 10 seconds
3. Generate IDs - duplicates may occur

Affected Versions: All versions < 1.2.0

Suggested Fix:
Add stricter clock backward detection and fail-fast behavior.
```

## Response Timeline

- **Acknowledgment:** Within 48 hours
- **Initial assessment:** Within 5 business days
- **Status updates:** Every 7 days until resolved
- **Fix release:** Depends on severity
  - Critical: 1-7 days
  - High: 7-30 days
  - Medium: 30-90 days
  - Low: Next scheduled release

## Security Best Practices

When using this library:

### 1. Worker ID Management

```go
// ❌ DON'T: Use default generator in production
id, _ := snowflake.GenerateID()

// ✅ DO: Assign unique worker IDs
gen, _ := snowflake.New(getUniqueWorkerID())
id, _ := gen.GenerateID()
```

### 2. Clock Drift Monitoring

```go
// Monitor clock issues
metrics := gen.GetMetrics()
if metrics.ClockBackwardErr > 0 {
    // Alert: System clock issues detected
    alertOps("Clock drift detected")
}
```

### 3. Input Validation

```go
// Validate parsed IDs
id, err := snowflake.ParseString(userInput)
if err != nil {
    return ErrInvalidID
}
if !id.IsValid() {
    return ErrInvalidID
}
```

### 4. Rate Limiting

```go
// Prevent sequence overflow
if metrics.SequenceOverflow > threshold {
    // Too many IDs generated per millisecond
    return ErrRateLimitExceeded
}
```

## Known Security Considerations

### 1. Predictable IDs

Snowflake IDs are **sequential and predictable** by design. Consider:

- IDs reveal approximate generation time
- IDs expose worker/node information
- Sequence numbers are incrementing

**Mitigation:** If you need unpredictable IDs, use UUID v4 or encrypt the IDs.

### 2. Time-based Attacks

If system time can be manipulated:

- ID collisions may occur
- Monotonicity may break

**Mitigation:** Use NTP, monitor clock drift, set appropriate `MaxClockBackward`.

### 3. Worker ID Collisions

If two nodes share the same worker ID:

- ID collisions WILL occur
- Data integrity compromised

**Mitigation:** Use centralized worker ID assignment (Redis, Etcd).

## Public Disclosure

After a fix is released:

1. We will publish a security advisory
2. CVE will be requested if applicable
3. Credit will be given to the reporter (unless anonymity is requested)

## Questions?

For security questions (non-vulnerability): Open a public GitHub issue or discussion.

For vulnerabilities: Email security@yourdomain.com

---

Thank you for helping keep this project secure!

# Conformance

This directory is reserved for Beads backend conformance tests.

The intended upstream shape is:

```go
func TestConformance(t *testing.T) {
    conformance.RunAll(t, func(t *testing.T) storage.Store {
        return newDoltLitePluginStore(t)
    })
}
```

For an external process plugin, the test harness should open the plugin through
the same client transport core Beads uses. That keeps conformance focused on the
public plugin contract instead of private implementation details.

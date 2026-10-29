package plugin

type Capability int64

const (
	CapabilityNone  Capability = 0
	CapabilityText  Capability = 1 << 0
	CapabilityMedia Capability = 1 << 1
)

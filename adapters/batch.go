package adapters

// DefaultValue marks a batch slot whose record omitted the column. The VALUES
// list renders DEFAULT there; executors drop the marker before binding.
type DefaultValue struct{}

// BoundValues returns values without DefaultValue markers, in order.
func BoundValues(values []interface{}) []interface{} {
	if !HasDefaults(values) {
		return values
	}
	out := make([]interface{}, 0, len(values))
	for _, v := range values {
		if _, ok := v.(DefaultValue); ok {
			continue
		}
		out = append(out, v)
	}
	return out
}

// HasDefaults reports whether any slot is a DefaultValue (COPY cannot express DEFAULT).
func HasDefaults(values []interface{}) bool {
	for _, v := range values {
		if _, ok := v.(DefaultValue); ok {
			return true
		}
	}
	return false
}

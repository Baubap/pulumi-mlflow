package client

// KeyValue is MLflow's generic tag / key-value representation, shared by
// experiment, registered-model and model-version tag endpoints.
type KeyValue struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// TagsToKeyValues converts a tag map into MLflow's key/value tag array. It
// returns nil for an empty map so the field is omitted from request bodies.
func TagsToKeyValues(tags map[string]string) []KeyValue {
	if len(tags) == 0 {
		return nil
	}
	kvs := make([]KeyValue, 0, len(tags))
	for k, v := range tags {
		kvs = append(kvs, KeyValue{Key: k, Value: v})
	}
	return kvs
}

// TagsToKV is an alias for TagsToKeyValues.
func TagsToKV(tags map[string]string) []KeyValue { return TagsToKeyValues(tags) }

// KeyValuesToMap converts an MLflow key/value tag array into a map. It returns
// nil for an empty slice.
func KeyValuesToMap(kvs []KeyValue) map[string]string {
	if len(kvs) == 0 {
		return nil
	}
	m := make(map[string]string, len(kvs))
	for _, kv := range kvs {
		m[kv.Key] = kv.Value
	}
	return m
}

// KVToMap is an alias for KeyValuesToMap.
func KVToMap(kvs []KeyValue) map[string]string { return KeyValuesToMap(kvs) }

// DiffTags compares desired tags against the current state and returns the tags
// to set (added or changed) and the keys to remove. Callers apply upserts via
// the entity's set-tag endpoint and removals via delete-tag (where supported).
func DiffTags(current, desired map[string]string) (upsert map[string]string, remove []string) {
	upsert = map[string]string{}
	for k, v := range desired {
		if cur, ok := current[k]; !ok || cur != v {
			upsert[k] = v
		}
	}
	for k := range current {
		if _, ok := desired[k]; !ok {
			remove = append(remove, k)
		}
	}
	return upsert, remove
}

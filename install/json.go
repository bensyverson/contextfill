package install

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// member is one key of a JSON object, its value kept as written.
type member struct {
	Key   string
	Value json.RawMessage
}

// object is a JSON object whose keys keep their file order, which a Go map
// would sort.
type object []member

// parseObject reads a JSON object, keeping its keys in order and its values
// unparsed.
func parseObject(data []byte) (object, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if tok != json.Delim('{') {
		return nil, errors.New("not a JSON object")
	}
	var o object
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, _ := tok.(string)
		var value json.RawMessage
		if err := dec.Decode(&value); err != nil {
			return nil, err
		}
		o = append(o, member{Key: key, Value: value})
	}
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, errors.New("more than one JSON value")
	}
	return o, nil
}

// get returns the value at key, if the object has it.
func (o object) get(key string) (json.RawMessage, bool) {
	for _, m := range o {
		if m.Key == key {
			return m.Value, true
		}
	}
	return nil, false
}

// set replaces the value at key in place, or appends the key.
func (o object) set(key string, value json.RawMessage) object {
	for i := range o {
		if o[i].Key == key {
			o[i].Value = value
			return o
		}
	}
	return append(o, member{Key: key, Value: value})
}

// without returns the object less the key.
func (o object) without(key string) object {
	var out object
	for _, m := range o {
		if m.Key != key {
			out = append(out, m)
		}
	}
	return out
}

// encode writes the object compactly, values as they were.
func (o object) encode() json.RawMessage {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, m := range o {
		if i > 0 {
			b.WriteByte(',')
		}
		b.Write(encodeValue(m.Key))
		b.WriteByte(':')
		b.Write(m.Value)
	}
	b.WriteByte('}')
	return b.Bytes()
}

// encodeArray writes the values as a JSON array.
func encodeArray(values []json.RawMessage) json.RawMessage {
	var b bytes.Buffer
	b.WriteByte('[')
	for i, v := range values {
		if i > 0 {
			b.WriteByte(',')
		}
		b.Write(v)
	}
	b.WriteByte(']')
	return b.Bytes()
}

// encodeValue marshals v without escaping <, > and &, which belong in a
// shell command as written.
func encodeValue(v any) json.RawMessage {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		panic(fmt.Sprintf("install: encoding %T: %v", v, err))
	}
	return bytes.TrimSuffix(b.Bytes(), []byte("\n"))
}

// pretty indents the object the way Claude Code writes its settings: two
// spaces, and a final newline.
func pretty(o object) ([]byte, error) {
	var b bytes.Buffer
	if err := json.Indent(&b, o.encode(), "", "  "); err != nil {
		return nil, err
	}
	b.WriteByte('\n')
	return b.Bytes(), nil
}

package middlewares

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strconv"

	"github.com/vmihailenco/msgpack/v5"
)

// The msgpack renderer (_renderer=msgpack) encodes the response as
// MessagePack, a binary format with the same data model as JSON that is
// smaller and faster to parse. Objects become maps with their keys in the
// order of the JSON body, arrays become arrays, and strings, numbers,
// booleans and null become their msgpack counterparts (see
// encodeMsgpackNumber for how numbers are encoded):
//
//	[{"name":"a","age":1}]  ->  91 82 a4 6e 61 6d 65 a1 61 a3 61 67 65 01
const (
	msgpackRenderer = "msgpack"
	// msgpackContentType is the media type registered for MessagePack with
	// IANA.
	msgpackContentType = "application/vnd.msgpack"
	// maxMsgpackDepth bounds how deeply nested a body can be, as
	// encoding/json bounds what it decodes, so a pathological body cannot
	// exhaust the stack.
	maxMsgpackDepth = 10000
)

var (
	errMsgpackTooDeep        = errors.New("response is nested too deeply to encode as msgpack")
	errMsgpackTrailingData   = errors.New("response has data after its JSON value")
	errMsgpackNumberTooLarge = errors.New("response has a number out of the range of a float64")
)

// jsonNode is one value of a JSON document. A scalar keeps the token the
// decoder read (string, json.Number, bool or nil); an object or array keeps
// its delimiter and its elements in document order, an object's keys and
// values interleaved, so its keys keep their order and a repeated key (two
// aggregates both named "sum") keeps both values.
type jsonNode struct {
	token json.Token
	elems []jsonNode
}

// renderMsgpack encodes the JSON body as MessagePack. ok is false when body
// is not a single JSON value (an empty or plain-text body) or holds a number
// msgpack cannot represent; the caller then returns body unchanged.
func renderMsgpack(body []byte) (out []byte, ok bool) {
	node, err := parseJSONNode(body)
	if err != nil {
		return nil, false
	}
	var buf bytes.Buffer
	if err := node.encodeMsgpack(msgpack.NewEncoder(&buf)); err != nil {
		return nil, false
	}
	return buf.Bytes(), true
}

// parseJSONNode reads body, which must hold exactly one JSON value. Numbers
// are kept as json.Number so integers are not routed through float64.
func parseJSONNode(body []byte) (jsonNode, error) {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	node, err := readJSONNode(dec, 0)
	if err != nil {
		return jsonNode{}, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return jsonNode{}, errMsgpackTrailingData
	}
	return node, nil
}

func readJSONNode(dec *json.Decoder, depth int) (jsonNode, error) {
	tok, err := dec.Token()
	if err != nil {
		return jsonNode{}, err
	}
	if _, isDelim := tok.(json.Delim); !isDelim {
		return jsonNode{token: tok}, nil
	}
	// Token only returns an opening delimiter here: a closing one ends the
	// loop below, and the decoder rejects one that closes nothing.
	if depth >= maxMsgpackDepth {
		return jsonNode{}, errMsgpackTooDeep
	}
	node := jsonNode{token: tok, elems: []jsonNode{}}
	for dec.More() {
		elem, err := readJSONNode(dec, depth+1)
		if err != nil {
			return jsonNode{}, err
		}
		node.elems = append(node.elems, elem)
	}
	// Consume the closing delimiter.
	if _, err := dec.Token(); err != nil {
		return jsonNode{}, err
	}
	return node, nil
}

// encodeMsgpack writes the node with the smallest msgpack type that holds it.
func (n jsonNode) encodeMsgpack(enc *msgpack.Encoder) error {
	switch v := n.token.(type) {
	case json.Delim:
		var err error
		if v == '{' {
			err = enc.EncodeMapLen(len(n.elems) / 2)
		} else {
			err = enc.EncodeArrayLen(len(n.elems))
		}
		if err != nil {
			return err
		}
		for _, elem := range n.elems {
			if err := elem.encodeMsgpack(enc); err != nil {
				return err
			}
		}
		return nil
	case string:
		return enc.EncodeString(v)
	case json.Number:
		return encodeMsgpackNumber(enc, v)
	case bool:
		return enc.EncodeBool(v)
	default: // nil: a JSON null
		return enc.EncodeNil()
	}
}

// encodeMsgpackNumber writes an integer that fits in 64 bits as a msgpack
// integer and any other number (a fraction, an exponent, an integer wider
// than 64 bits) as a float64, which is how JSON decoders read such numbers
// too. A number beyond the range of a float64 is an error rather than an
// infinity.
func encodeMsgpackNumber(enc *msgpack.Encoder, n json.Number) error {
	if i, err := strconv.ParseInt(n.String(), 10, 64); err == nil {
		return enc.EncodeInt(i)
	}
	if u, err := strconv.ParseUint(n.String(), 10, 64); err == nil {
		return enc.EncodeUint(u)
	}
	f, err := n.Float64()
	if err != nil {
		return errMsgpackNumberTooLarge
	}
	return enc.EncodeFloat64(f)
}

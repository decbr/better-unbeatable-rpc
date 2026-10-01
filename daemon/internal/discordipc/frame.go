package discordipc

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

type Opcode uint32

const (
	OpHandshake Opcode = 0
	OpFrame     Opcode = 1
	OpClose     Opcode = 2
	OpPing      Opcode = 3
	OpPong      Opcode = 4
)

const maxPayload = 1 << 20

var ErrPayloadTooLarge = errors.New("discordipc: frame payload too large")

func WriteFrame(w io.Writer, op Opcode, payload []byte) error {
	if len(payload) > maxPayload {
		return ErrPayloadTooLarge
	}
	buf := make([]byte, 8+len(payload))
	binary.LittleEndian.PutUint32(buf[0:4], uint32(op))
	binary.LittleEndian.PutUint32(buf[4:8], uint32(len(payload)))
	copy(buf[8:], payload)
	_, err := w.Write(buf)
	return err
}

func ReadFrame(r io.Reader) (Opcode, []byte, error) {
	var hdr [8]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return 0, nil, err
	}
	op := Opcode(binary.LittleEndian.Uint32(hdr[0:4]))
	n := binary.LittleEndian.Uint32(hdr[4:8])
	if n > maxPayload {
		return 0, nil, fmt.Errorf("%w: %d bytes", ErrPayloadTooLarge, n)
	}
	payload := make([]byte, n)
	if _, err := io.ReadFull(r, payload); err != nil {
		return 0, nil, err
	}
	return op, payload, nil
}

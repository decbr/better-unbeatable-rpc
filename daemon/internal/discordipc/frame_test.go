package discordipc

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"testing"
)

func TestFrameRoundTrip(t *testing.T) {
	cases := []struct {
		op      Opcode
		payload []byte
	}{
		{OpHandshake, []byte(`{"v":1,"client_id":"123"}`)},
		{OpFrame, []byte(`{"cmd":"SET_ACTIVITY","nonce":"x"}`)},
		{OpClose, []byte(`{}`)},
		{OpPing, nil},
		{OpPong, []byte("ünïcödé ★")},
	}
	var buf bytes.Buffer
	for _, c := range cases {
		if err := WriteFrame(&buf, c.op, c.payload); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range cases {
		op, payload, err := ReadFrame(&buf)
		if err != nil {
			t.Fatal(err)
		}
		if op != c.op || !bytes.Equal(payload, c.payload) && len(c.payload) > 0 {
			t.Errorf("got (%d, %q), want (%d, %q)", op, payload, c.op, c.payload)
		}
	}
	if _, _, err := ReadFrame(&buf); !errors.Is(err, io.EOF) {
		t.Errorf("expected EOF after last frame, got %v", err)
	}
}

func TestFrameHeaderIsLittleEndian(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteFrame(&buf, OpFrame, []byte("abc")); err != nil {
		t.Fatal(err)
	}
	want := []byte{1, 0, 0, 0, 3, 0, 0, 0, 'a', 'b', 'c'}
	if !bytes.Equal(buf.Bytes(), want) {
		t.Errorf("got % x, want % x", buf.Bytes(), want)
	}
}

func TestReadFrameRejectsHugeLength(t *testing.T) {
	var hdr [8]byte
	binary.LittleEndian.PutUint32(hdr[0:4], uint32(OpFrame))
	binary.LittleEndian.PutUint32(hdr[4:8], 0xFFFFFFFF)
	if _, _, err := ReadFrame(bytes.NewReader(hdr[:])); !errors.Is(err, ErrPayloadTooLarge) {
		t.Errorf("got %v, want ErrPayloadTooLarge", err)
	}
}

func TestReadFrameTruncated(t *testing.T) {
	var buf bytes.Buffer
	_ = WriteFrame(&buf, OpFrame, []byte("hello"))
	truncated := buf.Bytes()[:buf.Len()-2]
	if _, _, err := ReadFrame(bytes.NewReader(truncated)); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Errorf("got %v, want ErrUnexpectedEOF", err)
	}
}

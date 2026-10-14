package macros

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"reflect"
	"testing"
)

func TestX6TimedCapture(t *testing.T) {
	tests := []struct {
		name     string
		events   []X6TimedEvent
		pairs    []byte
		checksum uint16
	}{
		{"a", []X6TimedEvent{{4, Down, 10}, {4, Up, 10}}, []byte{1, 4, 0x81, 4}, 0x8d},
		{"shift_a_100", []X6TimedEvent{{0xe1, Down, 100}, {4, Down, 100}, {4, Up, 100}, {0xe1, Up, 10}}, []byte{0xb, 0xe1, 0xb, 4, 0x8b, 4, 0x81, 0xe1}, 0x2f1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := EncodeX6TimedBlock(tt.events, 1, 0)
			if err != nil {
				t.Fatal(err)
			}
			if len(b) != 128 || b[25] != byte(len(tt.pairs)/2) || !bytes.Equal(b[26:26+len(tt.pairs)], tt.pairs) {
				t.Fatalf("unexpected block %x", b)
			}
			if binary.BigEndian.Uint16(b[126:]) != tt.checksum {
				t.Fatalf("checksum %x", b[126:])
			}
			for _, v := range b[26+len(tt.pairs) : 126] {
				if v != 0 {
					t.Fatal("nonzero unused payload")
				}
			}
		})
	}
}

func TestX6TimedDelayBoundaries(t *testing.T) {
	for _, tt := range []struct {
		ms   int64
		want []byte
	}{
		{0, []byte{1, 4}}, {4, []byte{1, 4}}, {5, []byte{1, 4}}, {14, []byte{1, 4}}, {15, []byte{3, 4}}, {10, []byte{1, 4}}, {100, []byte{0xb, 4}},
		{1270, []byte{0x7f, 4}}, {1271, []byte{7, 4, 6, 3}},
		{1280, []byte{9, 4, 6, 3}}, {1800, []byte{1, 4, 9, 3}},
		{1880, []byte{9, 4, 9, 3}}, {51199, []byte{0x15, 4, 0xff, 3}},
	} {
		t.Run(fmt.Sprint(tt.ms), func(t *testing.T) {
			b, err := EncodeX6TimedBlock([]X6TimedEvent{{4, Down, tt.ms}, {4, Up, 100}}, 1, 0)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(b[26:26+len(tt.want)], tt.want) || int(b[25]) != len(tt.want)/2+1 || !bytes.Equal(b[26+len(tt.want):28+len(tt.want)], []byte{0x81, 4}) {
				t.Fatalf("delay %d: %x", tt.ms, b)
			}
		})
	}
}

func TestX6TimedReject(t *testing.T) {
	cases := [][]X6TimedEvent{nil, {{4, Down, -1}}, {{4, Down, 51200}}, {{4, Down, 65535}}, {{4, Down, 65536}}, {{4, "invalid", 10}}}
	cases = append(cases, make([]X6TimedEvent, 47))
	for _, n := range []int{12, 46} {
		es := make([]X6TimedEvent, n)
		for i := range es {
			es[i] = X6TimedEvent{4, Down, 1271}
		}
		cases = append(cases, es)
	}
	for i, es := range cases {
		before := append([]X6TimedEvent(nil), es...)
		if b, err := EncodeX6TimedBlock(es, 1, 0); err == nil || b != nil {
			t.Fatalf("case %d admitted", i)
		}
		if !reflect.DeepEqual(es, before) {
			t.Fatal("rejection mutated input")
		}
	}
	for _, r := range []int{0, 256} {
		if b, err := EncodeX6TimedBlock([]X6TimedEvent{{4, Down, 10}}, r, 0); err == nil || b != nil {
			t.Fatal("repeat admitted")
		}
	}
	if b, err := EncodeX6TimedBlock([]X6TimedEvent{{4, Down, 10}}, 1, 3); err == nil || b != nil {
		t.Fatal("mode admitted")
	}
}

func TestX6TimedPayloadCapacity(t *testing.T) {
	es := make([]X6TimedEvent, 46)
	for i := range es {
		es[i] = X6TimedEvent{4, Down, 10}
	}
	for i := 0; i < 4; i++ {
		es[i].DelayMS = 1271
	}
	b, err := EncodeX6TimedBlock(es, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if b[25] != 50 {
		t.Fatal("expected 50 pairs")
	}
	if binary.BigEndian.Uint16(b[126:]) != x6Checksum(b) {
		t.Fatal("checksum overlaps data")
	}
	es[4].DelayMS = 1271
	if b, err := EncodeX6TimedBlock(es, 1, 0); err == nil || b != nil {
		t.Fatal("51 pairs admitted")
	}
}

func TestX6TimedMouseCompatibility(t *testing.T) {
	for i, button := range x6SequenceButtons {
		old, err := EncodeX6SequenceBlock(X6Sequence{Buttons: []EventType{button}, Repeat: 5})
		if err != nil {
			t.Fatal(err)
		}
		code := byte(0xf1 + i)
		got, err := EncodeX6TimedBlock([]X6TimedEvent{{code, Down, 0}, {code, Up, 0}}, 5, 0)
		if err != nil || !bytes.Equal(old, got) {
			t.Fatal("legacy mouse block changed", err)
		}
	}
}

func TestX6TimedOrientationAndLimits(t *testing.T) {
	es := []X6TimedEvent{{0xe0, Up, 1880}, {4, Down, 51199}}
	before := append([]X6TimedEvent(nil), es...)
	b, err := EncodeX6TimedBlock(es, 255, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b[26:32], []byte{0x89, 0xe0, 9, 3, 1, 4}) || b[25] != 3 || b[0] != 2 || b[4] != 255 {
		t.Fatalf("orientation/terminal: %x", b)
	}
	if !reflect.DeepEqual(es, before) {
		t.Fatal("success mutated input")
	}
	es = make([]X6TimedEvent, 11)
	for i := range es {
		es[i] = X6TimedEvent{4, Down, 1271}
	}
	b, err = EncodeX6TimedBlock(es, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if b[25] != 21 {
		t.Fatal("ten extensions rejected")
	}
}

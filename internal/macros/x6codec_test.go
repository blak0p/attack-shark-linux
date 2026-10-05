package macros

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("../../captures/0x09-macro", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestX6DestinationUpload(t *testing.T) {
	click := X6Click{MouseLeft, 1}
	body := fixture(t, "left-repeat-1.bin")
	for _, destination := range []byte{1, 2, 3, 5, 6, 7, 8} {
		want := X6Upload{Destination: destination, Click: click}
		reports, err := EncodeX6Upload(want)
		if err != nil {
			t.Fatal(err)
		}
		reconstructed := append(append(append([]byte{}, reports[0][4:]...), reports[1][4:]...), reports[2][4:12]...)
		if !bytes.Equal(reconstructed, body) {
			t.Fatal("destination changed body")
		}
		for i, header := range [][4]byte{{9, 0x40, destination, 0}, {9, 0x40, destination, 1}, {9, 0x0c, destination, 2}} {
			expected := make([]byte, 64)
			copy(expected, header[:])
			start := i * 60
			size := 60
			if i == 2 {
				size = 8
			}
			copy(expected[4:], body[start:start+size])
			if !bytes.Equal(expected, reports[i]) {
				t.Fatalf("destination %d report %d differs", destination, i)
			}
		}
		got, err := DecodeX6Upload(reports)
		if err != nil || got != want {
			t.Fatalf("roundtrip: %+v %v", got, err)
		}
		if destination != 5 {
			if _, err := DecodeX6Reports(reports); err == nil {
				t.Fatal("legacy accepted non05")
			}
		}
	}
	for _, destination := range []byte{0, 4, 9, 255} {
		if _, err := EncodeX6Upload(X6Upload{destination, click}); err == nil {
			t.Fatal("invalid destination")
		}
		reports, _ := EncodeX6Reports(click)
		for _, r := range reports {
			r[2] = destination
		}
		if _, err := DecodeX6Upload(reports); err == nil {
			t.Fatal("invalid decoded destination")
		}
	}
}

func TestX6DestinationStrictNegatives(t *testing.T) {
	fresh := func() [][]byte {
		r, err := EncodeX6Upload(X6Upload{6, X6Click{MouseLeft, 1}})
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	for _, mutate := range []func([][]byte){
		func(r [][]byte) { r[1][2] = 5 },
		func(r [][]byte) { r[0][0] = 8 },
		func(r [][]byte) { r[1][1] = 0x3f },
		func(r [][]byte) { r[2][3] = 1 },
		func(r [][]byte) { r[2][12] = 1 },
		func(r [][]byte) { r[2][11] ^= 1 },
		func(r [][]byte) { r[0][8] = 0; r[2][11]-- },
		func(r [][]byte) { r[0][31] = 0xf3; r[0][33] = 0xf3; r[2][11] += 4 },
		func(r [][]byte) { r[0][4] = 1; r[2][11]++ },
		func(r [][]byte) { r[0] = r[0][:63] },
		func(r [][]byte) { r[0] = append(r[0], 0) },
		func(r [][]byte) { r[0], r[1] = r[1], r[0] },
	} {
		r := fresh()
		mutate(r)
		if _, err := DecodeX6Upload(r); err == nil {
			t.Fatal("accepted malformed upload")
		}
	}
	for _, r := range [][][]byte{nil, fresh()[:2], append(fresh(), make([]byte, 64))} {
		if _, err := DecodeX6Upload(r); err == nil {
			t.Fatal("report count")
		}
	}
	for _, click := range []X6Click{{MouseLeft, 0}, {MouseLeft, 256}, {EventType("keyboard"), 1}} {
		if _, err := EncodeX6Upload(X6Upload{6, click}); err == nil {
			t.Fatal("invalid click")
		}
	}
}

func TestX6CapturedBytes(t *testing.T) {
	var contract struct {
		Sources []struct {
			Uploads []struct {
				Fixture    string
				ReportsHex []string `json:"reports_hex"`
			}
		}
	}
	if err := json.Unmarshal(fixture(t, "contract.json"), &contract); err != nil {
		t.Fatal(err)
	}
	for _, source := range contract.Sources {
		for _, upload := range source.Uploads {
			t.Run(upload.Fixture, func(t *testing.T) {
				b := fixture(t, upload.Fixture)
				button := MouseLeft
				if upload.Fixture == "right-repeat-1.bin" {
					button = MouseRight
				}
				want := X6Click{Button: button, Repeat: int(b[4])}
				encoded, err := EncodeX6Block(want)
				if err != nil || !bytes.Equal(encoded, b) {
					t.Fatalf("encode: %x, %v", encoded, err)
				}
				got, err := DecodeX6Block(b)
				if err != nil || got != want {
					t.Fatalf("decode: %+v, %v", got, err)
				}
				reports, err := EncodeX6Reports(want)
				if err != nil {
					t.Fatal(err)
				}
				for i, h := range upload.ReportsHex {
					captured, err := hex.DecodeString(h)
					if err != nil {
						t.Fatal(err)
					}
					if !bytes.Equal(reports[i], captured) {
						t.Fatalf("report %d differs", i)
					}
				}
				got, err = DecodeX6Reports(reports)
				if err != nil || got != want {
					t.Fatalf("reports decode: %+v, %v", got, err)
				}
			})
		}
	}
}

func TestX6RepeatRange(t *testing.T) {
	for _, button := range []EventType{MouseLeft, MouseRight} {
		for repeat := 1; repeat <= 255; repeat++ {
			want := X6Click{Button: button, Repeat: repeat}
			b, err := EncodeX6Block(want)
			if err != nil {
				t.Fatal(err)
			}
			if len(b) != 128 || int(b[4]) != repeat {
				t.Fatalf("repeat %d incorrectly encoded", repeat)
			}
			// Independent arithmetic protects extrapolated checksum carry at 255.
			sum := 0
			for _, v := range b[:126] {
				sum += int(v)
			}
			if int(binary.BigEndian.Uint16(b[126:])) != sum {
				t.Fatal("checksum mismatch")
			}
			got, err := DecodeX6Block(b)
			if err != nil || got != want {
				t.Fatalf("roundtrip %+v: %+v %v", want, got, err)
			}
		}
	}
	for _, c := range []X6Click{{MouseLeft, 0}, {MouseLeft, 256}, {MouseLeft, -1}, {EventType("keyboard"), 1}} {
		if _, err := EncodeX6Block(c); err == nil {
			t.Fatalf("accepted %+v", c)
		}
		if _, err := EncodeX6Reports(c); err == nil {
			t.Fatalf("reports accepted %+v", c)
		}
	}
}

func repairChecksum(b []byte) {
	var sum uint16
	for _, v := range b[:126] {
		sum += uint16(v)
	}
	binary.BigEndian.PutUint16(b[126:], sum)
}

func TestX6RejectBlock(t *testing.T) {
	base := fixture(t, "left-repeat-1.bin")
	for _, b := range [][]byte{nil, base[:127], append(append([]byte{}, base...), 0)} {
		if _, err := DecodeX6Block(b); err == nil {
			t.Fatal("accepted wrong length")
		}
	}
	bad := append([]byte{}, base...)
	bad[127] ^= 1
	if _, err := DecodeX6Block(bad); err == nil {
		t.Fatal("accepted corrupt checksum")
	}
	// Every byte other than the repeat, paired button codes and checksum is fixed.
	for offset := 0; offset < 126; offset++ {
		if offset == 4 || offset == 27 || offset == 29 {
			continue
		}
		t.Run(fmt.Sprint(offset), func(t *testing.T) {
			b := append([]byte{}, base...)
			b[offset] ^= 1
			repairChecksum(b)
			if _, err := DecodeX6Block(b); err == nil {
				t.Fatal("accepted unsupported layout with valid checksum")
			}
		})
	}
	for _, changes := range []map[int]byte{{4: 0}, {27: 0xf2}, {29: 0xf2}, {27: 0xf3, 29: 0xf3}} {
		b := append([]byte{}, base...)
		for i, v := range changes {
			b[i] = v
		}
		repairChecksum(b)
		if _, err := DecodeX6Block(b); err == nil {
			t.Fatal("accepted unknown button/repeat")
		}
	}
}

func TestX6RejectFraming(t *testing.T) {
	fresh := func() [][]byte {
		r, err := EncodeX6Reports(X6Click{MouseLeft, 1})
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	for _, r := range [][][]byte{nil, fresh()[:2], append(fresh(), make([]byte, 64))} {
		if _, err := DecodeX6Reports(r); err == nil {
			t.Fatal("accepted report count")
		}
	}
	for report := 0; report < 3; report++ {
		for offset := 0; offset < 4; offset++ {
			r := fresh()
			r[report][offset] ^= 1
			if _, err := DecodeX6Reports(r); err == nil {
				t.Fatal("accepted header")
			}
		}
		r := fresh()
		r[report] = r[report][:63]
		if _, err := DecodeX6Reports(r); err == nil {
			t.Fatal("accepted short report")
		}
		r = fresh()
		r[report] = append(r[report], 0)
		if _, err := DecodeX6Reports(r); err == nil {
			t.Fatal("accepted long report")
		}
	}
	for offset := 12; offset < 64; offset++ {
		r := fresh()
		r[2][offset] = 1
		if _, err := DecodeX6Reports(r); err == nil {
			t.Fatal("accepted padding")
		}
	}
	r := fresh()
	r[0], r[1] = r[1], r[0]
	if _, err := DecodeX6Reports(r); err == nil {
		t.Fatal("accepted reordered reports")
	}
	r = fresh()
	r[2][11] ^= 1
	if _, err := DecodeX6Reports(r); err == nil {
		t.Fatal("accepted framed corrupt checksum")
	}
	r = fresh()
	r[0][4] = 1
	r[2][11]++
	if _, err := DecodeX6Reports(r); err == nil {
		t.Fatal("accepted framed unknown layout")
	}
}

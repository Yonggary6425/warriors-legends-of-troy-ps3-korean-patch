package main

// UDF support: many PS3 ISO dumps are ISO9660 + Joliet + UDF bridge images.
// Tools such as Bandizip/WinRAR read the UDF tree, so after relocating or
// resizing a file we must update its UDF File Entry as well.

import (
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"strings"
	"unicode/utf16"
)

type uext struct {
	lb, n uint32 // partition-relative start lb, length in sectors
	pos   uint32 // physical partition lb
}

type udfEntry struct {
	part uint16
	lb   uint32
}

type udfVol struct {
	f       *os.File
	pstart  uint32
	pdSecs  []int64
	metaRef int // partition reference number of metadata map, -1 if none
	meta    []uext
	mirror  []uext
	files   map[string]udfEntry
	anchor  []byte
}

func udfCRC(b []byte) uint16 {
	var crc uint16
	for _, x := range b {
		crc ^= uint16(x) << 8
		for i := 0; i < 8; i++ {
			if crc&0x8000 != 0 {
				crc = crc<<1 ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

func udfSetTag(d []byte, crcLen int, loc uint32) {
	binary.LittleEndian.PutUint32(d[12:], loc)
	binary.LittleEndian.PutUint16(d[10:], uint16(crcLen))
	binary.LittleEndian.PutUint16(d[8:], udfCRC(d[16:16+crcLen]))
	var s byte
	for i := 0; i < 16; i++ {
		if i != 4 {
			s += d[i]
		}
	}
	d[4] = s
}

func (u *udfVol) sector(n int64) ([]byte, error) { return readAt(u.f, n*SEC, SEC) }

func mapExt(ext []uext, lb uint32) (int64, bool) {
	for _, e := range ext {
		if lb >= e.lb && lb < e.lb+e.n {
			return int64(e.pos + lb - e.lb), true
		}
	}
	return 0, false
}

// abs returns absolute sectors (main and mirror copies) of partition lb.
func (u *udfVol) abs(part uint16, lb uint32) ([]int64, error) {
	if u.metaRef >= 0 && int(part) == u.metaRef {
		p, ok := mapExt(u.meta, lb)
		if !ok {
			return nil, fmt.Errorf("UDF metadata lb %d 범위 밖", lb)
		}
		out := []int64{int64(u.pstart) + p}
		if m, ok := mapExt(u.mirror, lb); ok {
			out = append(out, int64(u.pstart)+m)
		}
		return out, nil
	}
	return []int64{int64(u.pstart) + int64(lb)}, nil
}

type udfFE struct {
	raw    []byte
	hdr    int
	lea    uint32
	lad    uint32
	adtype uint16
	ftype  byte
	size   uint64
}

func parseFE(b []byte) (*udfFE, error) {
	t := binary.LittleEndian.Uint16(b)
	fe := &udfFE{raw: b}
	switch t {
	case 261:
		fe.hdr = 176
		fe.lea = binary.LittleEndian.Uint32(b[168:])
		fe.lad = binary.LittleEndian.Uint32(b[172:])
	case 266:
		fe.hdr = 216
		fe.lea = binary.LittleEndian.Uint32(b[208:])
		fe.lad = binary.LittleEndian.Uint32(b[212:])
	default:
		return nil, fmt.Errorf("UDF File Entry 아님 (tag %d)", t)
	}
	if fe.hdr+int(fe.lea)+int(fe.lad) > len(b) {
		return nil, errors.New("UDF File Entry 손상")
	}
	fe.adtype = binary.LittleEndian.Uint16(b[16+18:]) & 7
	fe.ftype = b[16+11]
	fe.size = binary.LittleEndian.Uint64(b[56:])
	return fe, nil
}

func (fe *udfFE) ads() []byte {
	o := fe.hdr + int(fe.lea)
	return fe.raw[o : o+int(fe.lad)]
}

// extents returns (part, lb, bytes) for each allocation descriptor.
func (fe *udfFE) extents(icbPart uint16) [][3]uint32 {
	var out [][3]uint32
	a := fe.ads()
	switch fe.adtype {
	case 0:
		for p := 0; p+8 <= len(a); p += 8 {
			l := binary.LittleEndian.Uint32(a[p:])
			if l&0x3fffffff == 0 {
				break
			}
			out = append(out, [3]uint32{uint32(icbPart), binary.LittleEndian.Uint32(a[p+4:]), l & 0x3fffffff})
		}
	case 1:
		for p := 0; p+16 <= len(a); p += 16 {
			l := binary.LittleEndian.Uint32(a[p:])
			if l&0x3fffffff == 0 {
				break
			}
			out = append(out, [3]uint32{uint32(binary.LittleEndian.Uint16(a[p+8:])), binary.LittleEndian.Uint32(a[p+4:]), l & 0x3fffffff})
		}
	}
	return out
}

func (u *udfVol) readFE(part uint16, lb uint32) (*udfFE, []int64, error) {
	secs, err := u.abs(part, lb)
	if err != nil {
		return nil, nil, err
	}
	b, err := u.sector(secs[0])
	if err != nil {
		return nil, nil, err
	}
	fe, err := parseFE(b)
	return fe, secs, err
}

func (u *udfVol) readData(fe *udfFE, part uint16) ([]byte, error) {
	if fe.adtype == 3 {
		return append([]byte(nil), fe.ads()...), nil
	}
	var out []byte
	for _, e := range fe.extents(part) {
		secs, err := u.abs(uint16(e[0]), e[1])
		if err != nil {
			return nil, err
		}
		// extents in metadata partition may be fragmented; read sector by sector there
		if u.metaRef >= 0 && int(e[0]) == u.metaRef {
			var chunk []byte
			for i := uint32(0); i*SEC < e[2]; i++ {
				s, err := u.abs(uint16(e[0]), e[1]+i)
				if err != nil {
					return nil, err
				}
				b, err := u.sector(s[0])
				if err != nil {
					return nil, err
				}
				chunk = append(chunk, b...)
			}
			out = append(out, chunk[:e[2]]...)
			continue
		}
		b, err := readAt(u.f, secs[0]*SEC, int(e[2]))
		if err != nil {
			return nil, err
		}
		out = append(out, b...)
	}
	if uint64(len(out)) > fe.size {
		out = out[:fe.size]
	}
	return out, nil
}

func openUDF(f *os.File) (*udfVol, error) {
	u := &udfVol{f: f, metaRef: -1, files: map[string]udfEntry{}}
	a, err := u.sector(256)
	if err != nil {
		return nil, err
	}
	if binary.LittleEndian.Uint16(a) != 2 {
		return nil, nil // no UDF
	}
	u.anchor = a
	var lvd []byte
	for _, vds := range [][2]uint32{{binary.LittleEndian.Uint32(a[16:]), binary.LittleEndian.Uint32(a[20:])}, {binary.LittleEndian.Uint32(a[24:]), binary.LittleEndian.Uint32(a[28:])}} {
		for i := uint32(0); i < vds[0]/SEC; i++ {
			s := int64(vds[1] + i)
			b, err := u.sector(s)
			if err != nil {
				return nil, err
			}
			t := binary.LittleEndian.Uint16(b)
			if t == 8 || t == 0 {
				break
			}
			if t == 5 {
				u.pdSecs = append(u.pdSecs, s)
				u.pstart = binary.LittleEndian.Uint32(b[188:])
			}
			if t == 6 && lvd == nil {
				lvd = b
			}
		}
	}
	if lvd == nil || len(u.pdSecs) == 0 {
		return nil, errors.New("UDF 볼륨 정보를 읽을 수 없습니다")
	}
	nmaps := binary.LittleEndian.Uint32(lvd[268:])
	mp := lvd[440:]
	var metaLoc, mirLoc uint32 = 0xffffffff, 0xffffffff
	for i, p := uint32(0), 0; i < nmaps && p+2 <= len(mp); i++ {
		typ, l := mp[p], int(mp[p+1])
		if l == 0 {
			break
		}
		if typ == 2 && strings.HasPrefix(string(mp[p+5:p+28]), "*UDF Metadata Partition") {
			u.metaRef = int(i)
			metaLoc = binary.LittleEndian.Uint32(mp[p+40:])
			mirLoc = binary.LittleEndian.Uint32(mp[p+44:])
		}
		p += l
	}
	if u.metaRef >= 0 {
		load := func(loc uint32) ([]uext, error) {
			b, err := u.sector(int64(u.pstart + loc))
			if err != nil {
				return nil, err
			}
			fe, err := parseFE(b)
			if err != nil {
				return nil, err
			}
			var ext []uext
			var lb uint32
			for _, e := range fe.extents(0) {
				n := (e[2] + SEC - 1) / SEC
				ext = append(ext, uext{lb: lb, n: n, pos: e[1]})
				lb += n
			}
			return ext, nil
		}
		if u.meta, err = load(metaLoc); err != nil {
			return nil, fmt.Errorf("UDF metadata: %v", err)
		}
		if mirLoc != 0xffffffff && mirLoc != metaLoc {
			if m, err := load(mirLoc); err == nil {
				u.mirror = m
			}
		}
	}
	fsdLB := binary.LittleEndian.Uint32(lvd[248+4:])
	fsdPart := binary.LittleEndian.Uint16(lvd[248+8:])
	secs, err := u.abs(fsdPart, fsdLB)
	if err != nil {
		return nil, err
	}
	fsd, err := u.sector(secs[0])
	if err != nil {
		return nil, err
	}
	if binary.LittleEndian.Uint16(fsd) != 256 {
		return nil, errors.New("UDF File Set Descriptor를 찾을 수 없습니다")
	}
	rootLB := binary.LittleEndian.Uint32(fsd[400+4:])
	rootPart := binary.LittleEndian.Uint16(fsd[400+8:])
	if err := u.walk(rootPart, rootLB, "", 0); err != nil {
		return nil, err
	}
	return u, nil
}

func (u *udfVol) walk(part uint16, lb uint32, prefix string, depth int) error {
	if depth > 32 {
		return nil
	}
	fe, _, err := u.readFE(part, lb)
	if err != nil {
		return err
	}
	data, err := u.readData(fe, part)
	if err != nil {
		return err
	}
	for p := 0; p+38 <= len(data); {
		if binary.LittleEndian.Uint16(data[p:]) != 257 {
			break
		}
		ch := data[p+18]
		lfi := int(data[p+19])
		icb := binary.LittleEndian.Uint32(data[p+24:])
		ipart := binary.LittleEndian.Uint16(data[p+28:])
		liu := int(binary.LittleEndian.Uint16(data[p+36:]))
		if p+38+liu+lfi > len(data) {
			break
		}
		nb := data[p+38+liu : p+38+liu+lfi]
		p += (38 + liu + lfi + 3) &^ 3
		if ch&8 != 0 || lfi == 0 || ch&4 != 0 {
			continue
		}
		nm := ""
		if nb[0] == 8 {
			nm = string(nb[1:])
		} else {
			w := make([]uint16, (lfi-1)/2)
			for i := range w {
				w[i] = binary.BigEndian.Uint16(nb[1+2*i:])
			}
			nm = string(utf16.Decode(w))
		}
		full := prefix + strings.ToUpper(nm)
		if ch&2 != 0 {
			if err := u.walk(ipart, icb, full+"/", depth+1); err != nil {
				return err
			}
		} else {
			u.files[full] = udfEntry{ipart, icb}
		}
	}
	return nil
}

func (u *udfVol) read(path string) ([]byte, error) {
	e, ok := u.files[strings.ToUpper(path)]
	if !ok {
		return nil, fmt.Errorf("UDF에 파일 없음: %s", path)
	}
	fe, _, err := u.readFE(e.part, e.lb)
	if err != nil {
		return nil, err
	}
	return u.readData(fe, e.part)
}

// setExtent points path's File Entry at a single extent (absolute lba, size).
func (u *udfVol) setExtent(path string, lba uint32, size uint32) error {
	e, ok := u.files[strings.ToUpper(path)]
	if !ok {
		return fmt.Errorf("UDF에 파일 없음: %s", path)
	}
	fe, secs, err := u.readFE(e.part, e.lb)
	if err != nil {
		return err
	}
	if lba < u.pstart {
		return errors.New("UDF 파티션 밖 위치")
	}
	exts := fe.extents(e.part)
	if len(exts) == 0 || fe.adtype > 1 {
		return fmt.Errorf("지원하지 않는 UDF 할당 방식: %s", path)
	}
	if fe.adtype == 0 && u.metaRef >= 0 && int(e.part) == u.metaRef {
		return fmt.Errorf("지원하지 않는 UDF 할당 방식: %s", path)
	}
	d := fe.raw
	adOff := fe.hdr + int(fe.lea)
	var adLen int
	rel := lba - u.pstart
	if fe.adtype == 0 {
		adLen = 8
		binary.LittleEndian.PutUint32(d[adOff:], size)
		binary.LittleEndian.PutUint32(d[adOff+4:], rel)
	} else {
		adLen = 16
		pref := binary.LittleEndian.Uint16(d[adOff+8:])
		for i := adOff; i < adOff+16; i++ {
			d[i] = 0
		}
		binary.LittleEndian.PutUint32(d[adOff:], size)
		binary.LittleEndian.PutUint32(d[adOff+4:], rel)
		binary.LittleEndian.PutUint16(d[adOff+8:], pref)
	}
	for i := adOff + adLen; i < adOff+int(fe.lad); i++ {
		d[i] = 0
	}
	blocks := uint64((size + SEC - 1) / SEC)
	binary.LittleEndian.PutUint64(d[56:], uint64(size))
	if fe.hdr == 216 {
		binary.LittleEndian.PutUint64(d[64:], uint64(size)) // object size
		binary.LittleEndian.PutUint64(d[72:], blocks)
		binary.LittleEndian.PutUint32(d[212:], uint32(adLen))
	} else {
		binary.LittleEndian.PutUint64(d[64:], blocks)
		binary.LittleEndian.PutUint32(d[172:], uint32(adLen))
	}
	loc := binary.LittleEndian.Uint32(d[12:])
	udfSetTag(d, adOff+adLen-16, loc)
	for _, s := range secs {
		if _, err := u.f.WriteAt(d, s*SEC); err != nil {
			return err
		}
	}
	return nil
}

// grow extends the partition to cover newTotal sectors and writes a copy of
// the anchor in the last sector.
func (u *udfVol) grow(newTotal uint32, anchor bool) error {
	last := newTotal - 1
	need := last // the last sector always holds an anchor copy
	for _, s := range u.pdSecs {
		b, err := u.sector(s)
		if err != nil {
			return err
		}
		if binary.LittleEndian.Uint32(b[188:])+binary.LittleEndian.Uint32(b[192:]) >= need {
			continue
		}
		binary.LittleEndian.PutUint32(b[192:], need-u.pstart)
		udfSetTag(b, int(binary.LittleEndian.Uint16(b[10:])), uint32(s))
		if _, err := u.f.WriteAt(b, s*SEC); err != nil {
			return err
		}
	}
	if !anchor {
		return nil
	}
	a := append([]byte(nil), u.anchor...)
	udfSetTag(a, int(binary.LittleEndian.Uint16(a[10:])), last)
	_, err := u.f.WriteAt(a, int64(last)*SEC)
	return err
}

// udfCheck returns the UDF paths whose current content is one of the known
// versions; other files are left alone.
func (u *udfVol) start(path string) (uint32, bool) {
	e, ok := u.files[strings.ToUpper(path)]
	if !ok {
		return 0, false
	}
	fe, _, err := u.readFE(e.part, e.lb)
	if err != nil || fe.adtype > 1 {
		return 0, false
	}
	x := fe.extents(e.part)
	if len(x) == 0 || (u.metaRef >= 0 && int(x[0][0]) == u.metaRef) {
		return 0, false
	}
	return u.pstart + x[0][1], true
}

func udfKnown(u *udfVol, pf pfile, isoLBA uint32) bool {
	if l, ok := u.start(pf.path); ok && l == isoLBA {
		return true
	}
	b, err := u.read(pf.path)
	if err != nil {
		return false
	}
	h := sha1.Sum(b)
	if h == pf.newSHA || h == pf.origSHA {
		return true
	}
	for _, set := range sets {
		for _, q := range set {
			if q.path == pf.path && (q.origSHA == h || q.newSHA == h) {
				return true
			}
		}
	}
	return false
}

// extent returns the absolute start lba and size of path in the UDF tree.
func (u *udfVol) extent(path string) (uint32, uint64, bool) {
	e, ok := u.files[strings.ToUpper(path)]
	if !ok {
		return 0, 0, false
	}
	fe, _, err := u.readFE(e.part, e.lb)
	if err != nil || fe.adtype > 1 {
		return 0, 0, false
	}
	x := fe.extents(e.part)
	if len(x) == 0 || (u.metaRef >= 0 && int(x[0][0]) == u.metaRef) {
		return 0, 0, false
	}
	return u.pstart + x[0][1], fe.size, true
}

// syncFromISO points every UDF file entry that disagrees with the ISO9660
// directory at the ISO9660 extent (only single-extent files).
func (u *udfVol) syncFromISO(files map[string][]rec) (fixed, failed int) {
	for p := range u.files {
		rs := files[p]
		if len(rs) == 0 {
			continue
		}
		l, sz, ok := u.extent(p)
		if ok && l == rs[0].lba && sz == uint64(rs[0].size) {
			continue
		}
		if err := u.setExtent(p, rs[0].lba, rs[0].size); err != nil {
			failed++
		} else {
			fixed++
		}
	}
	return
}

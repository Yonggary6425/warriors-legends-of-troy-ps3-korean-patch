package main

import (
	"bufio"
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	_ "embed"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf16"
)

//go:embed patch.bin
var patchData []byte

//go:embed patch_upd.bin
var patchUpd []byte

// edition: "full" (원본/v1.0.1 -> 최신) or "update" (v1.0.1 -> 최신)
var edition = "full"

var version = "1.0.2"

const SEC = 2048

var titleID = "BLES01183"
var gameName = "Warriors: Legends of Troy"
var prevVersion = "1.0.1"

type op struct {
	kind   byte
	off, n uint64
	data   []byte
}
type pfile struct {
	path     string
	origSize uint64
	origSHA  [20]byte
	newSize  uint64
	newSHA   [20]byte
	ops      []op
}

func loadPatch(data []byte) ([]pfile, error) {
	if len(data) == 0 {
		return nil, nil
	}
	zr, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	raw, err := io.ReadAll(zr)
	if err != nil {
		return nil, err
	}
	r := bytes.NewReader(raw)
	magic := make([]byte, 8)
	io.ReadFull(r, magic)
	if string(magic) != "TROYKO01" {
		return nil, errors.New("패치 데이터가 손상되었습니다")
	}
	var n uint32
	binary.Read(r, binary.LittleEndian, &n)
	files := make([]pfile, n)
	for i := range files {
		f := &files[i]
		var pl uint16
		binary.Read(r, binary.LittleEndian, &pl)
		pb := make([]byte, pl)
		io.ReadFull(r, pb)
		f.path = string(pb)
		binary.Read(r, binary.LittleEndian, &f.origSize)
		io.ReadFull(r, f.origSHA[:])
		binary.Read(r, binary.LittleEndian, &f.newSize)
		io.ReadFull(r, f.newSHA[:])
		var no uint32
		binary.Read(r, binary.LittleEndian, &no)
		f.ops = make([]op, no)
		for j := range f.ops {
			o := &f.ops[j]
			o.kind, _ = r.ReadByte()
			if o.kind == 0 {
				binary.Read(r, binary.LittleEndian, &o.off)
				binary.Read(r, binary.LittleEndian, &o.n)
			} else {
				binary.Read(r, binary.LittleEndian, &o.n)
				o.data = make([]byte, o.n)
				io.ReadFull(r, o.data)
			}
		}
	}
	return files, nil
}

// ---------------- ISO9660 ----------------
type rec struct {
	off  int64 // absolute byte offset of directory record
	lba  uint32
	size uint32
}
type iso struct {
	f     *os.File
	vds   []int64 // offsets of PVD/SVD sectors
	files map[string][]rec
	total uint32
}

func readAt(f *os.File, off int64, n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := f.ReadAt(b, off)
	return b, err
}

func decodeName(b []byte, joliet bool) string {
	var s string
	if joliet {
		u := make([]uint16, len(b)/2)
		for i := range u {
			u[i] = binary.BigEndian.Uint16(b[2*i:])
		}
		s = string(utf16.Decode(u))
	} else {
		s = string(b)
	}
	if i := strings.IndexByte(s, ';'); i >= 0 {
		s = s[:i]
	}
	s = strings.TrimSuffix(s, ".")
	return strings.ToUpper(s)
}

func (is *iso) walk(lba, size uint32, prefix string, joliet bool, depth int) error {
	if depth > 32 {
		return nil
	}
	data, err := readAt(is.f, int64(lba)*SEC, int(size))
	if err != nil {
		return err
	}
	p := 0
	for p < len(data) {
		l := int(data[p])
		if l == 0 {
			p = (p/SEC + 1) * SEC
			continue
		}
		if p+l > len(data) || l < 34 {
			break
		}
		r := data[p : p+l]
		elba := binary.LittleEndian.Uint32(r[2:])
		esz := binary.LittleEndian.Uint32(r[10:])
		flags := r[25]
		nl := int(r[32])
		name := r[33 : 33+nl]
		if !(nl == 1 && (name[0] == 0 || name[0] == 1)) {
			nm := decodeName(name, joliet)
			full := prefix + nm
			if flags&2 != 0 {
				if err := is.walk(elba, esz, full+"/", joliet, depth+1); err != nil {
					return err
				}
			} else {
				is.files[full] = append(is.files[full], rec{off: int64(lba)*SEC + int64(p), lba: elba, size: esz})
			}
		}
		p += l
	}
	return nil
}

func openISO(path string) (*iso, error) {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	is := &iso{f: f, files: map[string][]rec{}}
	for s := int64(16); s < 64; s++ {
		vd, err := readAt(f, s*SEC, SEC)
		if err != nil {
			return nil, err
		}
		if string(vd[1:6]) != "CD001" {
			if string(vd[1:6]) == "BEA01" || string(vd[1:6]) == "NSR02" || string(vd[1:6]) == "NSR03" {
				return nil, errors.New("UDF 형식 ISO는 지원하지 않습니다")
			}
			continue
		}
		t := vd[0]
		if t == 255 {
			break
		}
		if t == 1 || t == 2 {
			joliet := t == 2 && vd[88] == '%' && vd[89] == '/'
			if t == 2 && !joliet {
				continue
			}
			is.vds = append(is.vds, s*SEC)
			if t == 1 {
				is.total = binary.LittleEndian.Uint32(vd[80:])
			}
			root := vd[156:190]
			if err := is.walk(binary.LittleEndian.Uint32(root[2:]), binary.LittleEndian.Uint32(root[10:]), "", joliet, 0); err != nil {
				return nil, err
			}
		}
	}
	if len(is.vds) == 0 {
		return nil, errors.New("ISO9660 형식이 아닙니다")
	}
	return is, nil
}

func (is *iso) read(path string) ([]byte, []rec, error) {
	rs := is.files[strings.ToUpper(path)]
	if len(rs) == 0 {
		return nil, nil, fmt.Errorf("ISO 안에서 파일을 찾을 수 없습니다: %s", path)
	}
	b, err := readAt(is.f, int64(rs[0].lba)*SEC, int(rs[0].size))
	return b, rs, err
}

func put32both(f *os.File, off int64, v uint32) error {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint32(b, v)
	binary.BigEndian.PutUint32(b[4:], v)
	_, err := f.WriteAt(b, off)
	return err
}

func paramTitle(sfo []byte) string {
	if len(sfo) < 20 || string(sfo[1:4]) != "PSF" {
		return ""
	}
	ks := binary.LittleEndian.Uint32(sfo[8:])
	ds := binary.LittleEndian.Uint32(sfo[12:])
	n := binary.LittleEndian.Uint32(sfo[16:])
	for i := uint32(0); i < n; i++ {
		e := sfo[20+16*i:]
		ko := binary.LittleEndian.Uint16(e[0:])
		ln := binary.LittleEndian.Uint32(e[4:])
		do := binary.LittleEndian.Uint32(e[12:])
		k := sfo[ks+uint32(ko):]
		k = k[:bytes.IndexByte(k, 0)]
		if string(k) == "TITLE_ID" {
			return strings.TrimRight(string(sfo[ds+do:ds+do+ln]), "\x00")
		}
	}
	return ""
}

// ---------------- UI helpers ----------------
var in = bufio.NewReader(os.Stdin)

func ask(prompt string) string {
	fmt.Print(prompt)
	s, _ := in.ReadString('\n')
	return strings.TrimSpace(s)
}

func pause() {
	fmt.Println()
	ask("Enter 키를 누르면 종료합니다...")
}

func copyFile(src, dst string) error {
	s, err := os.Open(src)
	if err != nil {
		return err
	}
	defer s.Close()
	st, _ := s.Stat()
	d, err := os.Create(dst)
	if err != nil {
		return err
	}
	buf := make([]byte, 8<<20)
	var done int64
	last := -1
	for {
		n, er := s.Read(buf)
		if n > 0 {
			if _, ew := d.Write(buf[:n]); ew != nil {
				d.Close()
				return ew
			}
			done += int64(n)
			pc := int(done * 100 / st.Size())
			if pc != last {
				fmt.Printf("\r  복사 중... %3d%%", pc)
				last = pc
			}
		}
		if er == io.EOF {
			break
		}
		if er != nil {
			d.Close()
			return er
		}
	}
	fmt.Println()
	return d.Close()
}

func run() error {
	fmt.Println("==============================================")
	if edition == "update" {
		fmt.Printf(" %s (%s) 한글패치\n 업데이트 v%s → v%s\n", gameName, titleID, prevVersion, version)
	} else {
		fmt.Printf(" %s (%s) 한글패치 v%s\n", gameName, titleID, version)
	}
	fmt.Println("==============================================")
	files, err := loadAll()
	if err != nil {
		return err
	}
	src := ""
	if len(os.Args) > 1 {
		src = os.Args[1]
	} else {
		src = ask("\nISO 파일 또는 압축을 푼 게임 폴더(PS3_GAME 폴더가 들어 있는 폴더)를\n이 창에 끌어다 놓거나 경로를 입력한 뒤 Enter:\n> ")
	}
	src = strings.Trim(src, "\"' ")
	st0, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("파일이나 폴더를 찾을 수 없습니다: %s", src)
	}
	if st0.IsDir() {
		return runFolder(files, src)
	}

	// check before copying
	chk, err := openISO(src)
	if err != nil {
		return err
	}
	sfo, _, err := chk.read("PS3_GAME/PARAM.SFO")
	if err != nil {
		chk.f.Close()
		return err
	}
	if t := paramTitle(sfo); t != titleID {
		chk.f.Close()
		if t == "" {
			return fmt.Errorf("PARAM.SFO를 읽을 수 없습니다. 암호화된 ISO이거나 손상된 파일입니다.\n복호화된(decrypted) ISO에만 적용할 수 있습니다.")
		}
		return fmt.Errorf("이 패치는 %s 전용입니다. (이 ISO: %s)", titleID, t)
	}
	// check originals
	files, err = resolve(func(p string) ([]byte, error) { b, _, e := chk.read(p); return b, e })
	if err != nil {
		chk.f.Close()
		return err
	}
	state := "patched"
	for _, pf := range files {
		b, _, err := chk.read(pf.path)
		if err != nil {
			chk.f.Close()
			return err
		}
		h := sha1.Sum(b)
		if h == pf.newSHA && uint64(len(b)) == pf.newSize {
			continue
		}
		state = "orig"
		if h != pf.origSHA {
			chk.f.Close()
			return mismatchMsg(pf.path, "ISO")
		}
	}
	chk.f.Close()
	if state == "patched" {
		fmt.Println("\n이미 최신 한글패치가 적용된 ISO입니다. (파일 목록만 점검합니다)")
	}

	fmt.Println("\n[1] 새 ISO 파일로 저장 (권장, 원본 유지)")
	fmt.Println("[2] 원본 ISO에 바로 적용 (빠름, 원본이 바뀜)")
	mode := ask("선택 (1/2, 기본 1): ")
	target := src
	if mode != "2" {
		ext := filepath.Ext(src)
		target = strings.TrimSuffix(src, ext) + " [KOR]" + ext
		fmt.Printf("\n새 파일: %s\n", target)
		if err := copyFile(src, target); err != nil {
			return fmt.Errorf("복사 실패: %v", err)
		}
	}

	is, err := openISO(target)
	if err != nil {
		return err
	}
	defer is.f.Close()
	uv, uerr := openUDF(is.f)
	if uerr != nil {
		fmt.Println("\n※ UDF 정보를 읽지 못해 ISO9660/Joliet만 고칩니다:", uerr)
		uv = nil
	}
	udfOK := map[string]bool{}
	if uv != nil {
		for _, pf := range files {
			rs := is.files[strings.ToUpper(pf.path)]
			_, inUDF := uv.files[strings.ToUpper(pf.path)]
			udfOK[pf.path] = len(rs) > 0 && inUDF
		}
	}
	final := map[string][2]uint32{}
	st, _ := is.f.Stat()
	end := (st.Size() + SEC - 1) / SEC * SEC
	oldTotal := uint32(end / SEC)
	if is.total > oldTotal {
		oldTotal = is.total
		end = int64(oldTotal) * SEC
	}

	// read all originals first
	type job struct {
		pf  pfile
		old []byte
		rs  []rec
	}
	var jobs []job
	for _, pf := range files {
		b, rs, err := is.read(pf.path)
		if err != nil {
			return err
		}
		if sha1.Sum(b) == pf.newSHA {
			continue
		}
		jobs = append(jobs, job{pf, b, rs})
	}
	fmt.Printf("\n파일 %d개 적용 중...\n", len(jobs))
	for i, j := range jobs {
		var nb []byte
		for _, o := range j.pf.ops {
			if o.kind == 0 {
				nb = append(nb, j.old[o.off:o.off+o.n]...)
			} else {
				nb = append(nb, o.data...)
			}
		}
		if sha1.Sum(nb) != j.pf.newSHA {
			return fmt.Errorf("패치 결과 검증 실패: %s", j.pf.path)
		}
		alloc := (uint64(j.rs[0].size) + SEC - 1) / SEC * SEC
		lba := j.rs[0].lba
		if uint64(len(nb)) > alloc {
			lba = uint32(end / SEC)
			end += (int64(len(nb)) + SEC - 1) / SEC * SEC
		}
		pad := make([]byte, (len(nb)+SEC-1)/SEC*SEC)
		copy(pad, nb)
		if _, err := is.f.WriteAt(pad, int64(lba)*SEC); err != nil {
			return err
		}
		final[j.pf.path] = [2]uint32{lba, uint32(len(nb))}
		for _, r := range j.rs {
			if err := put32both(is.f, r.off+2, lba); err != nil {
				return err
			}
			if err := put32both(is.f, r.off+10, uint32(len(nb))); err != nil {
				return err
			}
		}
		fmt.Printf("\r  %d / %d", i+1, len(jobs))
	}
	fmt.Println()
	anchor := false
	if uv != nil {
		lb, _ := readAt(is.f, end-SEC, SEC)
		if len(lb) < 16 || binary.LittleEndian.Uint16(lb) != 2 || binary.LittleEndian.Uint32(lb[12:]) != uint32(end/SEC-1) {
			end += SEC // UDF anchor copy in the last sector
			anchor = true
		}
	}
	newTotal := uint32(end / SEC)
	if newTotal != oldTotal {
		for _, vd := range is.vds {
			if err := put32both(is.f, vd+80, newTotal); err != nil {
				return err
			}
		}
		// PS3 disc region table (sector 0)
		s0, _ := readAt(is.f, 0, SEC)
		n := binary.BigEndian.Uint32(s0)
		if n > 0 && n < 128 {
			lastEnd := 8 + int(n-1)*8 + 4
			if binary.BigEndian.Uint32(s0[lastEnd:]) == oldTotal-1 {
				b := make([]byte, 4)
				binary.BigEndian.PutUint32(b, newTotal-1)
				is.f.WriteAt(b, int64(lastEnd))
			}
		}
		if err := is.f.Truncate(end); err != nil {
			return err
		}
	}
	if uv != nil {
		if err := uv.grow(newTotal, anchor); err != nil {
			return fmt.Errorf("UDF 갱신 실패: %v", err)
		}
		skipped := 0
		for _, pf := range files {
			if !udfOK[pf.path] {
				skipped++
				continue
			}
			v, ok := final[pf.path]
			if !ok {
				rs := is.files[strings.ToUpper(pf.path)]
				v = [2]uint32{rs[0].lba, rs[0].size}
			}
			if err := uv.setExtent(pf.path, v[0], v[1]); err != nil {
				udfOK[pf.path] = false
				skipped++
			}
		}
		// files patched earlier without UDF support (v1.0~v1.0.1): mirror ISO9660
		// reopen the ISO9660 tree to get current records
		if cur, err := openISO(target); err == nil {
			_, failed := uv.syncFromISO(cur.files)
			cur.f.Close()
			skipped += failed
		}
		if skipped > 0 {
			fmt.Printf("\n※ UDF 목록 %d개는 고치지 못했습니다. 반디집 등으로 압축을 풀면\n   그 파일은 원본으로 나올 수 있습니다. (ISO 실행에는 영향 없음)\n", skipped)
		}
	}
	is.f.Sync()

	// verify by re-reading through the directory tree
	is.f.Close()
	vf, err := openISO(target)
	if err != nil {
		return err
	}
	defer vf.f.Close()
	for _, pf := range files {
		b, _, err := vf.read(pf.path)
		if err != nil || sha1.Sum(b) != pf.newSHA {
			return fmt.Errorf("적용 후 검증 실패: %s", pf.path)
		}
	}
	if uv != nil {
		vu, err := openUDF(vf.f)
		if err != nil || vu == nil {
			return fmt.Errorf("적용 후 UDF 검증 실패: %v", err)
		}
		for _, pf := range files {
			if !udfOK[pf.path] {
				continue
			}
			b, err := vu.read(pf.path)
			if err != nil || sha1.Sum(b) != pf.newSHA {
				return fmt.Errorf("적용 후 UDF 검증 실패: %s", pf.path)
			}
		}
	}
	fmt.Println("\n완료! 한글패치가 적용되었습니다.")
	fmt.Println(target)
	return nil
}

func main() {
	setupConsole()
	if err := run(); err != nil {
		fmt.Println("\n[오류]", err)
	}
	pause()
}

var sets [][]pfile // candidate patch sets, all ending in the same final files
var fullSet []pfile

func loadAll() ([]pfile, error) {
	full, err := loadPatch(patchData)
	if err != nil {
		return nil, err
	}
	upd, err := loadPatch(patchUpd)
	if err != nil {
		return nil, err
	}
	if edition == "update" {
		sets = [][]pfile{upd}
	} else {
		sets = [][]pfile{full, upd}
		fullSet = full
	}
	if len(sets[0]) == 0 {
		return nil, errors.New("패치 데이터가 없습니다")
	}
	return sets[0], nil
}

// resolve picks, for every target file, the patch entry that matches the
// file's current content (original, previous version, or already final).
func resolve(read func(string) ([]byte, error)) ([]pfile, error) {
	out := make([]pfile, 0, len(sets[0]))
	for _, base := range sets[0] {
		b, err := read(base.path)
		if err != nil {
			return nil, err
		}
		h := sha1.Sum(b)
		pick := base
		if !(h == base.newSHA && uint64(len(b)) == base.newSize) {
			for _, set := range sets {
				for _, pf := range set {
					if pf.path == base.path && pf.origSHA == h && pf.newSHA == base.newSHA {
						pick = pf
					}
				}
			}
		}
		out = append(out, pick)
	}
	return out, nil
}

func mismatchMsg(path, where string) error {
	if edition == "update" {
		return fmt.Errorf("v%s 상태가 아닌 파일이 있습니다: %s\n이 업데이트 패처는 v%s을 적용한 %s 전용입니다.\n처음 적용하신다면 전체 패처(v%s_full)를 쓰세요.", prevVersion, path, prevVersion, where, version)
	}
	return fmt.Errorf("원본과 다른 파일이 있어 적용할 수 없습니다: %s\n(다른 판본이거나 이미 수정된 %s일 수 있습니다)", path, where)
}
